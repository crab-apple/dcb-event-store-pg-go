package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) Append(ctx context.Context, commands ...eventstore.AppendCommand) (eventstore.SequencePosition, error) {
	for _, cmd := range commands {
		if cmd.Condition != nil {
			if err := eventstore.ValidateAppendCondition(*cmd.Condition); err != nil {
				return eventstore.SequencePosition{}, err
			}
		}
	}

	totalEvents := 0
	leaf := map[int64]struct{}{}
	intent := map[int64]struct{}{}
	for _, cmd := range commands {
		totalEvents += len(cmd.Events)
		keys := computeWriterLockKeys(cmd.Events, cmd.Condition)
		for _, k := range keys.leafX {
			leaf[k] = struct{}{}
		}
		for _, k := range keys.intentS {
			intent[k] = struct{}{}
		}
	}
	if totalEvents == 0 {
		return eventstore.SequencePosition{}, fmt.Errorf("cannot append zero events")
	}

	keys := writerLockKeys{leafX: keysOf(leaf), intentS: keysOf(intent)}
	if totalEvents <= s.copyThreshold {
		return s.appendViaFunction(ctx, commands, keys)
	}
	return s.appendViaCopy(ctx, commands, keys, totalEvents)
}

// appendViaFunction does the whole append in one stored-procedure call, with
// every event passed as an array parameter.
func (s *Store) appendViaFunction(ctx context.Context, commands []eventstore.AppendCommand, keys writerLockKeys) (eventstore.SequencePosition, error) {
	types, tags, payloads := serializeEvents(commands)
	condIdxs, condTypes, condTags, condAfter := serializeConditions(commands)

	var pos int64
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.appendFunctionName),
		keys.leafX, keys.intentS, types, tags, payloads, condIdxs, condTypes, condTags, condAfter,
	).Scan(&pos)
	if err != nil {
		if idx, ok := parseConditionViolation(err); ok {
			return eventstore.SequencePosition{}, conditionError(commands, idx)
		}
		return eventstore.SequencePosition{}, err
	}

	return int64ToPosition(pos), nil
}

// appendViaCopy streams the events in with COPY instead of passing them as
// array parameters, for appends above the copy threshold. The steps the
// stored procedure performs internally become separate statements here, so
// they need an explicit transaction to stay atomic.
func (s *Store) appendViaCopy(
	ctx context.Context,
	commands []eventstore.AppendCommand,
	keys writerLockKeys,
	totalEvents int,
) (eventstore.SequencePosition, error) {
	// Read Committed is set explicitly rather than inherited from the server
	// default: the condition check has to see events that committed while we
	// were waiting for our locks.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return eventstore.SequencePosition{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock-then-allocate, as in the stored procedure -- COPY allocates positions.
	if _, err := tx.Exec(ctx, acquireWriterLocksSQL, keys.leafX, keys.intentS); err != nil {
		return eventstore.SequencePosition{}, err
	}

	var hwm int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT COALESCE(pg_sequence_last_value(pg_get_serial_sequence('%s', 'sequence_position')), 0)`,
		s.tableName,
	)).Scan(&hwm); err != nil {
		return eventstore.SequencePosition{}, err
	}

	// Checked before the COPY, as the stored procedure does: the check only
	// looks at positions up to hwm, so inserting first would just mean
	// discarding the rows again on a violation.
	idx, violated, err := s.checkConditions(ctx, tx, commands, hwm)
	if err != nil {
		return eventstore.SequencePosition{}, err
	}
	if violated {
		return eventstore.SequencePosition{}, conditionError(commands, idx)
	}

	events := make([]eventstore.Event, 0, totalEvents)
	for _, cmd := range commands {
		events = append(events, cmd.Events...)
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{s.tableName},
		[]string{"type", "tags", "payload"},
		pgx.CopyFromSlice(len(events), func(i int) ([]any, error) {
			return []any{events[i].Type, events[i].Tags.Values(), serializePayload(events[i])}, nil
		}),
	); err != nil {
		return eventstore.SequencePosition{}, err
	}

	var pos int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT currval(pg_get_serial_sequence('%s', 'sequence_position'))`, s.tableName,
	)).Scan(&pos); err != nil {
		return eventstore.SequencePosition{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, s.notifyChannel, strconv.FormatInt(pos, 10)); err != nil {
		return eventstore.SequencePosition{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return eventstore.SequencePosition{}, err
	}
	return int64ToPosition(pos), nil
}

// acquireWriterLocksSQL takes the leaf keys exclusively and the intent keys
// shared. Both modes go in one key-ordered statement, so writers with
// partially overlapping scopes can't deadlock against each other.
const acquireWriterLocksSQL = `
	SELECT CASE WHEN excl THEN pg_advisory_xact_lock(k) ELSE pg_advisory_xact_lock_shared(k) END
	FROM (
		SELECT k, true AS excl FROM unnest($1::bigint[]) k
		UNION ALL
		SELECT k, false AS excl FROM unnest($2::bigint[]) k
	) t
	ORDER BY k`

// checkConditions reports the index of the first command whose condition
// matches an event at or below hwm.
func (s *Store) checkConditions(
	ctx context.Context,
	tx pgx.Tx,
	commands []eventstore.AppendCommand,
	hwm int64,
) (int, bool, error) {
	idxs, types, tags, after := serializeConditions(commands)
	if len(idxs) == 0 {
		return 0, false, nil
	}

	var failed *int32
	if err := tx.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s($1,$2,$3,$4,$5)`, s.checkFunctionName),
		idxs, types, tags, after, hwm,
	).Scan(&failed); err != nil {
		return 0, false, err
	}
	if failed == nil {
		return 0, false, nil
	}
	return int(*failed), true, nil
}

func conditionError(commands []eventstore.AppendCommand, idx int) error {
	var commandIndex *int
	if len(commands) > 1 {
		commandIndex = &idx
	}
	return &eventstore.AppendConditionError{
		AppendCondition: *commands[idx].Condition,
		CommandIndex:    commandIndex,
	}
}

// parseConditionViolation reports whether err is the events_append function
// raising APPEND_CONDITION_VIOLATED, and if so, which command index failed.
func parseConditionViolation(err error) (int, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return 0, false
	}
	const prefix = "APPEND_CONDITION_VIOLATED:cmd="
	if !strings.HasPrefix(pgErr.Message, prefix) {
		return 0, false
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(pgErr.Message, prefix))
	if err != nil {
		return 0, false
	}
	return idx, true
}
