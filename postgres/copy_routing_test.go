package postgres

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two append paths are meant to be indistinguishable by behaviour, so
// routing is pinned by removing the stored procedure: afterwards only an
// append that takes the COPY path can still succeed.
func TestAppend_RoutesByCopyThreshold(t *testing.T) {
	pool := NewTestPool(t)
	store, err := New(pool, Options{CopyThreshold: 2})
	require.NoError(t, err)
	require.NoError(t, store.EnsureInstalled(t.Context()))

	_, err = pool.Exec(t.Context(), `DROP FUNCTION events_append`)
	require.NoError(t, err)

	t.Run("above the threshold", func(t *testing.T) {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: events(3)})
		assert.NoError(t, err)
	})

	t.Run("at the threshold", func(t *testing.T) {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: events(2)})
		assert.ErrorContains(t, err, "events_append")
	})
}

func events(n int) []eventstore.Event {
	evs := make([]eventstore.Event, n)
	for i := range evs {
		evs[i] = eventstore.Event{Type: "A"}
	}
	return evs
}
