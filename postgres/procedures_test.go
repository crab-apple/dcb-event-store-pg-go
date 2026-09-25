package postgres_test

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newInstalledPool returns a pool for a fresh database with the schema
// already installed.
func newInstalledPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := newTestPool(t)
	store, err := postgres.New(pool, postgres.Options{})
	require.NoError(t, err)
	require.NoError(t, store.EnsureInstalled(t.Context()))
	return pool
}

// appendCall mirrors events_append's parameter list.
type appendCall struct {
	Types    []string
	Tags     []string
	Payloads []string

	CondIdxs  []int32
	CondTypes []string
	CondTags  []string
	CondAfter []int64
}

func appendViaFunction(t *testing.T, pool *pgxpool.Pool, call appendCall) (int64, error) {
	t.Helper()
	var pos int64
	err := pool.QueryRow(t.Context(),
		`SELECT events_append($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		[]int64{}, []int64{}, call.Types, call.Tags, call.Payloads,
		call.CondIdxs, call.CondTypes, call.CondTags, call.CondAfter,
	).Scan(&pos)
	return pos, err
}

func TestAppendFunction_InsertsAndReturnsPosition(t *testing.T) {
	// Given
	pool := newInstalledPool(t)

	// When
	pos, err := appendViaFunction(t, pool, appendCall{
		Types:    []string{"A"},
		Tags:     []string{"tag-1"},
		Payloads: []string{"{}"},
	})

	// Then
	require.NoError(t, err)
	assert.Equal(t, int64(1), pos)

	var count int
	err = pool.QueryRow(t.Context(), `SELECT count(*) FROM events`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestAppendFunction_ConditionSucceedsWhenNothingMatches(t *testing.T) {
	// Given
	pool := newInstalledPool(t)

	// When
	pos, err := appendViaFunction(t, pool, appendCall{
		Types:     []string{"A"},
		Tags:      []string{"tag-1"},
		Payloads:  []string{"{}"},
		CondIdxs:  []int32{0},
		CondTypes: []string{"A"},
		CondTags:  []string{"other-tag"},
		CondAfter: []int64{0},
	})

	// Then
	require.NoError(t, err)
	assert.Equal(t, int64(1), pos)
}

func TestAppendFunction_ConditionFailsWhenEventMatches(t *testing.T) {
	// Given
	pool := newInstalledPool(t)
	_, err := appendViaFunction(t, pool, appendCall{
		Types:    []string{"A"},
		Tags:     []string{"tag-1"},
		Payloads: []string{"{}"},
	})
	require.NoError(t, err)

	// When
	_, err = appendViaFunction(t, pool, appendCall{
		Types:     []string{"A"},
		Tags:      []string{"tag-1"},
		Payloads:  []string{"{}"},
		CondIdxs:  []int32{0},
		CondTypes: []string{"A"},
		CondTags:  []string{"tag-1"},
		CondAfter: []int64{0},
	})

	// Then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APPEND_CONDITION_VIOLATED")
}

func TestAppendFunction_ConditionFailsWithMultipleConditionRows(t *testing.T) {
	// Given
	pool := newInstalledPool(t)
	_, err := appendViaFunction(t, pool, appendCall{
		Types:    []string{"B"},
		Tags:     []string{"tag-2"},
		Payloads: []string{"{}"},
	})
	require.NoError(t, err)

	// When
	// Two condition rows: the first (type A / nonexistent tag) doesn't
	// match, the second (type B / tag-2) does.
	_, err = appendViaFunction(t, pool, appendCall{
		Types:     []string{"A"},
		Tags:      []string{"tag-1"},
		Payloads:  []string{"{}"},
		CondIdxs:  []int32{0, 0},
		CondTypes: []string{"A", "B"},
		CondTags:  []string{"nonexistent", "tag-2"},
		CondAfter: []int64{0, 0},
	})

	// Then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APPEND_CONDITION_VIOLATED")
}

func TestBarrierFunction_ReturnsCurrentHighWaterMark(t *testing.T) {
	// Given
	pool := newInstalledPool(t)
	pos, err := appendViaFunction(t, pool, appendCall{
		Types:    []string{"A"},
		Tags:     []string{"tag-1"},
		Payloads: []string{"{}"},
	})
	require.NoError(t, err)

	// When
	var hwm int64
	err = pool.QueryRow(t.Context(), `SELECT events_barrier_hwm($1, $2)`, []int64{}, []int64{}).Scan(&hwm)

	// Then
	require.NoError(t, err)
	assert.Equal(t, pos, hwm)
}
