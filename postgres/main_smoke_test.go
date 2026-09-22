package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestPool_IsolatesDatabases(t *testing.T) {
	poolA := newTestPool(t)
	poolB := newTestPool(t)

	_, err := poolA.Exec(t.Context(), `CREATE TABLE marker (id int)`)
	require.NoError(t, err)

	var exists bool
	err = poolB.QueryRow(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'marker')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "poolB should not see poolA's table")
}
