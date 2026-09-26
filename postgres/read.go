package postgres

import (
	"context"
	"fmt"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

func (s *Store) Read(ctx context.Context, query eventstore.Query, opts eventstore.ReadOptions) (eventstore.EventIterator, error) {
	// Backwards reads scan from the highest position downward, so they can't
	// advance past an invisible gap and skip the barrier entirely.
	var upperBound *int64
	if !opts.Backwards {
		hwm, err := s.barrierSnapshot(ctx, query)
		if err != nil {
			return nil, err
		}
		upperBound = &hwm
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		conn.Release()
		return nil, err
	}

	sql, args := buildReadSQL(s.tableName, query, opts, upperBound)
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
		conn.Release()
		return nil, err
	}

	return &cursorIterator{ctx: ctx, conn: conn}, nil
}

// barrierSnapshot acquires the reader-side barrier locks for query, snapshots
// the high-water mark, and releases them -- see docs/postgres/design.md#the-read-barrier.
func (s *Store) barrierSnapshot(ctx context.Context, query eventstore.Query) (int64, error) {
	keys := computeReaderLockKeys(query)
	var hwm int64
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT %s($1, $2)", s.barrierFunctionName),
		keys.leafS, keys.intentX,
	).Scan(&hwm)
	return hwm, err
}
