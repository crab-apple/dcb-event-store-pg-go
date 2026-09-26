// Most tests below duplicate eventstore/eventstoretest's Read suite, for the
// same reason as append_test.go (postgres.Store still can't satisfy
// eventstore.Store). The barrier and cursor-batching tests are
// Postgres-specific and stay regardless.
package postgres_test

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/crab-apple/dcb-event-store-pg-go/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustReadAll(
	t *testing.T,
	store *postgres.Store,
	query eventstore.Query,
	opts eventstore.ReadOptions,
) []eventstore.SequencedEvent {
	t.Helper()
	it, err := store.Read(t.Context(), query, opts)
	require.NoError(t, err)
	events, err := eventstore.Collect(it)
	require.NoError(t, err)
	return events
}

func eventTypes(events []eventstore.SequencedEvent) []string {
	types := make([]string, len(events))
	for i, ev := range events {
		types[i] = ev.Event.Type
	}
	return types
}

func TestRead_EmptyStoreYieldsNoEvents(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)

	// When
	events := mustReadAll(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})

	// Then
	assert.Empty(t, events)
}

func TestRead_YieldsEventsInAscendingOrder(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	for _, typ := range []string{"A", "B", "C"} {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{{Type: typ}}})
		require.NoError(t, err)
	}

	// When
	events := mustReadAll(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})

	// Then
	require.Len(t, events, 3)
	assert.Equal(t, []string{"A", "B", "C"}, eventTypes(events))
}

func TestRead_BackwardsYieldsDescendingOrder(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	for _, typ := range []string{"A", "B", "C"} {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{{Type: typ}}})
		require.NoError(t, err)
	}

	// When
	events := mustReadAll(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Backwards: true})

	// Then
	require.Len(t, events, 3)
	assert.Equal(t, []string{"C", "B", "A"}, eventTypes(events))
}

func TestRead_FiltersByTypeAndTag(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	for _, ev := range []eventstore.Event{
		{Type: "A", Tags: mustTags(t, "x")},
		{Type: "A", Tags: mustTags(t, "y")},
		{Type: "B", Tags: mustTags(t, "x")},
	} {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{ev}})
		require.NoError(t, err)
	}

	// When
	query := mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "x")})
	events := mustReadAll(t, store, query, eventstore.ReadOptions{})

	// Then
	require.Len(t, events, 1)
	assert.Equal(t, "A", events[0].Event.Type)
}

func TestRead_RespectsLimit(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	for range 3 {
		_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{{Type: "A"}}})
		require.NoError(t, err)
	}

	// When
	events := mustReadAll(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Limit: 2})

	// Then
	assert.Len(t, events, 2)
}

func TestRead_BarrierExcludesEventsAppendedAfterReadIsCalled(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{{Type: "A"}}})
	require.NoError(t, err)

	// When
	it, err := store.Read(t.Context(), eventstore.QueryAll(), eventstore.ReadOptions{})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	_, err = store.Append(t.Context(), eventstore.AppendCommand{Events: []eventstore.Event{{Type: "B"}}})
	require.NoError(t, err)

	// Then
	events, err := eventstore.Collect(it)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "A", events[0].Event.Type)
}

func TestRead_FetchesAcrossCursorBatches(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	const n = 5001 // exceeds the 5000-row FETCH batch size
	events := make([]eventstore.Event, n)
	for i := range events {
		events[i] = eventstore.Event{Type: "A"}
	}
	_, err := store.Append(t.Context(), eventstore.AppendCommand{Events: events})
	require.NoError(t, err)

	// When
	got := mustReadAll(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})

	// Then
	assert.Len(t, got, n)
}
