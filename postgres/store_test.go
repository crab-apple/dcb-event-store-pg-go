package postgres_test

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureInstalled_CreatesSchema(t *testing.T) {
	pool := newTestPool(t)
	store, err := postgres.New(pool, postgres.Options{})
	require.NoError(t, err)
	require.NoError(t, store.EnsureInstalled(t.Context()))

	var exists bool
	err = pool.QueryRow(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'events')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureInstalled_IsIdempotent(t *testing.T) {
	pool := newTestPool(t)
	store, err := postgres.New(pool, postgres.Options{})
	require.NoError(t, err)

	require.NoError(t, store.EnsureInstalled(t.Context()))
	require.NoError(t, store.EnsureInstalled(t.Context()))
}

func TestEnsureInstalled_RespectsTablePrefix(t *testing.T) {
	pool := newTestPool(t)
	store, err := postgres.New(pool, postgres.Options{TablePrefix: "custom"})
	require.NoError(t, err)
	require.NoError(t, store.EnsureInstalled(t.Context()))

	var exists bool
	err = pool.QueryRow(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'custom_events')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestNew_RejectsInvalidTablePrefix(t *testing.T) {
	_, err := postgres.New(nil, postgres.Options{TablePrefix: "bad prefix!"})
	assert.Error(t, err)
}
