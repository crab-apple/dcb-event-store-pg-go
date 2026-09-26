package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
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

	types, tags, payloads, condIdxs, condTypes, condTags, condAfter := serializeCommands(commands)

	var pos int64
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.appendFunctionName),
		keysOf(leaf), keysOf(intent), types, tags, payloads, condIdxs, condTypes, condTags, condAfter,
	).Scan(&pos)
	if err != nil {
		if idx, ok := parseConditionViolation(err); ok {
			var commandIndex *int
			if len(commands) > 1 {
				commandIndex = &idx
			}
			return eventstore.SequencePosition{}, &eventstore.AppendConditionError{
				AppendCondition: *commands[idx].Condition,
				CommandIndex:    commandIndex,
			}
		}
		return eventstore.SequencePosition{}, err
	}

	return int64ToPosition(pos), nil
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
