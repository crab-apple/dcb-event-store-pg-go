package eventstoretest

import (
	"slices"
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func comparePosition(a, b eventstore.SequencedEvent) int {
	return eventstore.CompareSequencePosition(a.Position, b.Position)
}

// seedABB appends three events -- type A tagged tag-1, type B tagged tag-2,
// type B tagged tag-3 -- and returns their positions in append order.
func seedABB(t *testing.T, store eventstore.Store) []eventstore.SequencePosition {
	t.Helper()
	var positions []eventstore.SequencePosition
	for _, ev := range []struct {
		eventType string
		tag       string
	}{
		{"A", "tag-1"},
		{"B", "tag-2"},
		{"B", "tag-3"},
	} {
		pos := mustAppendEvent(t, store, ev.eventType, []string{ev.tag})
		positions = append(positions, pos)
	}
	return positions
}

func testRead(t *testing.T, newStore NewStore) {
	t.Run("an empty store yields no events", func(t *testing.T) {
		for _, backwards := range []bool{false, true} {
			// Given
			store := newStore(t)

			// When
			events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Backwards: backwards})

			// Then
			assert.Empty(t, events)
		}
	})

	t.Run("yields events in ascending position order by default", func(t *testing.T) {
		// Given
		store := newStore(t)
		seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})

		// Then
		require.Len(t, events, 3)
		assert.True(t, slices.IsSortedFunc(events, comparePosition))
	})

	t.Run("backwards yields events in descending position order", func(t *testing.T) {
		// Given
		store := newStore(t)
		seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Backwards: true})

		// Then
		require.Len(t, events, 3)
		assert.True(t, slices.IsSortedFunc(events, func(a, b eventstore.SequencedEvent) int {
			return comparePosition(b, a)
		}))
	})

	t.Run("after excludes events at or before the given position", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{After: new(positions[1])})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[2], events[0].Position)
	})

	t.Run("after with backwards excludes events at or after the given position", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{
			After: new(positions[1]), Backwards: true,
		})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[0], events[0].Position)
	})

	t.Run("backwards with after at the first position yields nothing", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{
			After: new(positions[0]), Backwards: true,
		})

		// Then
		assert.Empty(t, events)
	})

	t.Run("filters by type", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		query, err := eventstore.QueryFromItems([]eventstore.QueryItem{{Types: []string{"B"}}})
		require.NoError(t, err)
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		require.Len(t, events, 2)
		assert.Equal(t, positions[1:], positionsOf(events))
	})

	t.Run("query items are combined with OR", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		query, err := eventstore.QueryFromItems([]eventstore.QueryItem{
			{Types: []string{"B"}},
			{Types: []string{"A"}},
		})
		require.NoError(t, err)
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		assert.Equal(t, positions, positionsOf(events))
	})

	t.Run("filters by an exact tag match", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		query := mustQuery(t, []string{"A", "B"}, []string{"tag-2"})
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[1], events[0].Position)
	})

	t.Run("no match when the tag differs", func(t *testing.T) {
		// Given
		store := newStore(t)
		seedABB(t, store)

		// When
		query := mustQuery(t, []string{"A", "B"}, []string{"unmatched"})
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		assert.Empty(t, events)
	})

	t.Run("matches when the event's tags are a superset of the filter's tags", func(t *testing.T) {
		// Given
		store := newStore(t)
		pos := mustAppendEvent(t, store, "A", []string{"tag-1", "tag-2"})

		// When
		query := mustQuery(t, []string{"A"}, []string{"tag-1"})
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, pos, events[0].Position)
	})

	t.Run("within a query item, type and tags are combined with AND", func(t *testing.T) {
		// Given
		store := newStore(t)
		mustAppendEvent(t, store, "A", []string{"tag-2"})
		mustAppendEvent(t, store, "B", []string{"tag-1"})

		// When
		query := mustQuery(t, []string{"A"}, []string{"tag-1"})
		events := mustRead(t, store, query, eventstore.ReadOptions{})

		// Then
		assert.Empty(t, events)
	})

	t.Run("limit caps events read forward to the earliest matches", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Limit: 1})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[0], events[0].Position)
	})

	t.Run("limit caps events read backward to the latest matches", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{Limit: 1, Backwards: true})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[2], events[0].Position)
	})

	t.Run("combines after and limit", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{After: new(positions[0]), Limit: 1})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[1], events[0].Position)
	})

	t.Run("combines after, type filter, and limit", func(t *testing.T) {
		// Given
		store := newStore(t)
		positions := seedABB(t, store)

		// When
		query, err := eventstore.QueryFromItems([]eventstore.QueryItem{{Types: []string{"B"}}})
		require.NoError(t, err)
		events := mustRead(t, store, query, eventstore.ReadOptions{After: new(positions[0]), Limit: 1})

		// Then
		require.Len(t, events, 1)
		assert.Equal(t, positions[1], events[0].Position)
	})
}

func positionsOf(events []eventstore.SequencedEvent) []eventstore.SequencePosition {
	positions := make([]eventstore.SequencePosition, len(events))
	for i, ev := range events {
		positions[i] = ev.Position
	}
	return positions
}
