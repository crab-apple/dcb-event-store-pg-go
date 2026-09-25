package postgres

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

var validIdentifier = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_]{0,62}$`)

// tagDelimiter separates an event's tags when they're packed into a single
// text parameter for the stored procedures below -- Postgres array
// parameters must be rectangular, so a per-event, variable-length tag list
// can't travel as a text[][].
const tagDelimiter = "\x1F"

// Store is a Postgres-backed eventstore.Store.
type Store struct {
	pool                *pgxpool.Pool
	tableName           string
	appendFunctionName  string
	barrierFunctionName string
	notifyChannel       string
}

// Options configures New.
type Options struct {
	// TablePrefix, if set, names the events table <prefix>_events instead of
	// the default "events".
	TablePrefix string
}

// New returns a Store backed by pool. Call EnsureInstalled before using it.
func New(pool *pgxpool.Pool, opts Options) (*Store, error) {
	tableName := "events"
	if opts.TablePrefix != "" {
		tableName = opts.TablePrefix + "_events"
	}
	if !validIdentifier.MatchString(tableName) {
		return nil, fmt.Errorf("invalid table name %q: must match %s", tableName, validIdentifier)
	}
	return &Store{
		pool:                pool,
		tableName:           tableName,
		appendFunctionName:  tableName + "_append",
		barrierFunctionName: tableName + "_barrier_hwm",
		notifyChannel:       tableName,
	}, nil
}

// migrationLockKey guards the schema migration below against concurrent
// callers; it shares no keyspace with the content lock keys computed in
// lockkeys.go. The exact value is arbitrary.
const migrationLockKey int64 = -89001

// EnsureInstalled creates the events table, its indexes, and its stored
// procedures if they don't already exist. Idempotent -- safe to call on
// every startup.
func (s *Store) EnsureInstalled(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockKey) }()

	if _, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %[1]s (
			sequence_position BIGSERIAL PRIMARY KEY,
			type              TEXT COLLATE "C" NOT NULL,
			tags              TEXT[] NOT NULL,
			payload           TEXT NOT NULL
		) WITH (
			autovacuum_freeze_min_age = 10000000,
			autovacuum_freeze_table_age = 100000000
		);

		CREATE INDEX IF NOT EXISTS %[1]s_type_pos_idx
		ON %[1]s (type COLLATE "C", sequence_position DESC);

		CREATE INDEX IF NOT EXISTS %[1]s_tags_gin
		ON %[1]s USING GIN(tags) WITH (fastupdate=off);
	`, s.tableName)); err != nil {
		return err
	}

	// The lock-then-allocate invariant is load-bearing for the read barrier:
	// locks (leaf X and intent S) are acquired before any INSERT.
	if _, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %[1]s(
			p_lock_keys      bigint[],
			p_intent_keys    bigint[],
			p_types          text[],
			p_tags           text[],
			p_payloads       text[],
			p_cond_cmd_idxs  int[],
			p_cond_types     text[],
			p_cond_tags      text[],
			p_cond_after     bigint[]
		) RETURNS bigint AS $fn$
		DECLARE
			v_hwm    bigint;
			v_pos    bigint;
			v_failed int;
		BEGIN
			IF (p_lock_keys IS NOT NULL AND array_length(p_lock_keys, 1) > 0)
			   OR (p_intent_keys IS NOT NULL AND array_length(p_intent_keys, 1) > 0) THEN
				PERFORM CASE WHEN excl THEN pg_advisory_xact_lock(k) ELSE pg_advisory_xact_lock_shared(k) END
				FROM (
					SELECT k, true AS excl FROM unnest(p_lock_keys) k
					UNION ALL
					SELECT k, false AS excl FROM unnest(p_intent_keys) k
				) t
				ORDER BY k;
			END IF;

			SELECT COALESCE(pg_sequence_last_value(pg_get_serial_sequence('%[2]s', 'sequence_position')), 0)
			INTO v_hwm;

			IF p_cond_cmd_idxs IS NOT NULL AND array_length(p_cond_cmd_idxs, 1) > 0 THEN
				IF array_length(p_cond_cmd_idxs, 1) = 1 THEN
					PERFORM 1 FROM %[2]s e
					WHERE e.type = p_cond_types[1]
					  AND e.tags @> CASE WHEN p_cond_tags[1] = '' THEN ARRAY[]::text[]
					                     ELSE string_to_array(p_cond_tags[1], '%[3]s') END
					  AND e.sequence_position > p_cond_after[1]
					  AND e.sequence_position <= v_hwm
					LIMIT 1;
					IF FOUND THEN
						RAISE EXCEPTION 'APPEND_CONDITION_VIOLATED:cmd=%%', p_cond_cmd_idxs[1];
					END IF;
				ELSE
					SET LOCAL enable_hashjoin = off;
					SET LOCAL enable_mergejoin = off;
					SET LOCAL plan_cache_mode = force_generic_plan;

					WITH conds AS MATERIALIZED (
						SELECT c.cmd_idx, c.ctype,
						       CASE WHEN c.ctags_str = '' THEN ARRAY[]::text[]
						            ELSE string_to_array(c.ctags_str, '%[3]s') END AS ctags,
						       c.after_pos
						FROM unnest(p_cond_cmd_idxs, p_cond_types, p_cond_tags, p_cond_after)
						     AS c(cmd_idx, ctype, ctags_str, after_pos)
					)
					SELECT c.cmd_idx INTO v_failed
					FROM conds c
					WHERE EXISTS (
						SELECT 1 FROM %[2]s e
						WHERE e.tags @> c.ctags
						  AND e.type = c.ctype
						  AND e.sequence_position > c.after_pos
						  AND e.sequence_position <= v_hwm
					)
					ORDER BY c.cmd_idx
					LIMIT 1;

					SET LOCAL enable_hashjoin = on;
					SET LOCAL enable_mergejoin = on;
					SET LOCAL plan_cache_mode = auto;

					IF v_failed IS NOT NULL THEN
						RAISE EXCEPTION 'APPEND_CONDITION_VIOLATED:cmd=%%', v_failed;
					END IF;
				END IF;
			END IF;

			INSERT INTO %[2]s (type, tags, payload)
			SELECT p_types[i], string_to_array(p_tags[i], '%[3]s'), p_payloads[i]
			FROM generate_subscripts(p_types, 1) AS i;

			SELECT currval(pg_get_serial_sequence('%[2]s', 'sequence_position')) INTO v_pos;
			PERFORM pg_notify('%[4]s', v_pos::text);
			RETURN v_pos;
		END;
		$fn$ LANGUAGE plpgsql;
	`, s.appendFunctionName, s.tableName, tagDelimiter, s.notifyChannel)); err != nil {
		return err
	}

	// Acquires reader-side barrier locks (S leaf, X intent), snapshots the
	// high-water mark, and returns it; the caller then scans with
	// sequence_position <= hwm. See docs/postgres/design.md#the-read-barrier.
	if _, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %[1]s(
			p_shared_keys    bigint[],
			p_exclusive_keys bigint[]
		) RETURNS bigint AS $br$
		DECLARE
			v_hwm bigint;
		BEGIN
			PERFORM CASE WHEN excl THEN pg_advisory_xact_lock(k) ELSE pg_advisory_xact_lock_shared(k) END
			FROM (
				SELECT k, false AS excl FROM unnest(p_shared_keys) k
				UNION ALL
				SELECT k, true AS excl FROM unnest(p_exclusive_keys) k
			) t
			ORDER BY k;

			SELECT COALESCE(pg_sequence_last_value(pg_get_serial_sequence('%[2]s', 'sequence_position')), 0)
			INTO v_hwm;
			RETURN v_hwm;
		END;
		$br$ LANGUAGE plpgsql;
	`, s.barrierFunctionName, s.tableName)); err != nil {
		return err
	}

	return nil
}
