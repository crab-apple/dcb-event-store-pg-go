package postgres

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

var validIdentifier = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_]{0,62}$`)

// Store is a Postgres-backed eventstore.Store.
type Store struct {
	pool      *pgxpool.Pool
	tableName string
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
	return &Store{pool: pool, tableName: tableName}, nil
}

// migrationLockKey guards the schema migration below against concurrent
// callers; it shares no keyspace with the content lock keys added in a later
// step.
const migrationLockKey int64 = -89001

// EnsureInstalled creates the events table and its indexes if they don't
// already exist. Idempotent -- safe to call on every startup.
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

	_, err = conn.Exec(ctx, fmt.Sprintf(`
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
	`, s.tableName))
	return err
}
