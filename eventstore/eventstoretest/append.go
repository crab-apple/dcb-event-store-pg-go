package eventstoretest

import (
	"errors"
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAppend(t *testing.T, newStore NewStore) {
	t.Run("single event append returns a position after initial", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		pos, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", nil)},
		})
		require.NoError(t, err)

		// Then
		assert.True(t, pos.IsAfter(eventstore.SequencePositionInitial()))
	})

	t.Run("sequential appends produce increasing positions", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		pos1, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", nil)},
		})
		require.NoError(t, err)
		pos2, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "B", nil)},
		})
		require.NoError(t, err)

		// Then
		assert.True(t, pos2.IsAfter(pos1))
	})

	t.Run("returned position is the last event of a multi-event command", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		pos, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", nil), mustEvent(t, "B", nil), mustEvent(t, "C", nil)},
		})
		require.NoError(t, err)

		// Then
		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})
		require.Len(t, events, 3)
		assert.Equal(t, pos, events[2].Position)
	})

	t.Run("rejects invalid append input", func(t *testing.T) {
		// Given
		tests := []struct {
			name     string
			commands []eventstore.AppendCommand
		}{
			{"zero commands", nil},
			{"command with zero events", []eventstore.AppendCommand{{}}},
			{"command with an invalid append condition", []eventstore.AppendCommand{{
				Events:    []eventstore.Event{mustEvent(t, "A", nil)},
				Condition: &eventstore.AppendCondition{FailIfEventsMatch: eventstore.QueryAll()},
			}}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				store := newStore(t)

				// When
				_, err := store.Append(t.Context(), tc.commands...)

				// Then
				require.Error(t, err)
				var conditionErr *eventstore.AppendConditionError
				assert.False(t, errors.As(err, &conditionErr))
			})
		}
	})

	t.Run("condition succeeds when nothing matches", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		_, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events:    []eventstore.Event{mustEvent(t, "A", []string{"tag-1"})},
			Condition: new(scopedCondition(t, []string{"A"}, []string{"tag-1"}, nil)),
		})

		// Then
		assert.NoError(t, err)
	})

	t.Run("condition fails when a matching event exists after the given position", func(t *testing.T) {
		// Given
		store := newStore(t)
		_, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", []string{"tag-1"})},
		})
		require.NoError(t, err)

		// When
		condition := scopedCondition(t, []string{"A"}, []string{"tag-1"}, nil)
		_, err = store.Append(t.Context(), eventstore.AppendCommand{
			Events:    []eventstore.Event{mustEvent(t, "A", []string{"tag-1"})},
			Condition: &condition,
		})

		// Then
		var conditionErr *eventstore.AppendConditionError
		require.ErrorAs(t, err, &conditionErr)
		assert.Nil(t, conditionErr.CommandIndex)
		assert.Equal(t, condition, conditionErr.AppendCondition)
	})

	t.Run("condition succeeds when the only matching event is at or before after", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", []string{"tag-1"})},
		})
		require.NoError(t, err)

		// When
		_, err = store.Append(t.Context(), eventstore.AppendCommand{
			Events:    []eventstore.Event{mustEvent(t, "B", []string{"tag-1"})},
			Condition: new(scopedCondition(t, []string{"A"}, []string{"tag-1"}, &posA)),
		})

		// Then
		assert.NoError(t, err)
	})

	t.Run("a multi-command append is atomic", func(t *testing.T) {
		// Given
		store := newStore(t)
		_, err := store.Append(t.Context(), eventstore.AppendCommand{
			Events: []eventstore.Event{mustEvent(t, "A", []string{"tag-1"})},
		})
		require.NoError(t, err)

		// When
		condition := scopedCondition(t, []string{"A"}, []string{"tag-1"}, nil)
		_, err = store.Append(t.Context(),
			eventstore.AppendCommand{Events: []eventstore.Event{mustEvent(t, "B", nil)}},
			eventstore.AppendCommand{
				Events:    []eventstore.Event{mustEvent(t, "C", nil)},
				Condition: &condition,
			},
		)

		// Then
		var conditionErr *eventstore.AppendConditionError
		require.ErrorAs(t, err, &conditionErr)
		require.NotNil(t, conditionErr.CommandIndex)
		assert.Equal(t, 1, *conditionErr.CommandIndex)
		assert.Equal(t, condition, conditionErr.AppendCondition)

		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})
		assert.Len(t, events, 1)
	})
}
