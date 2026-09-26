// Most tests below duplicate eventstore/eventstoretest's Append suite.
// Temporary: postgres.Store can't satisfy eventstore.Store yet (no Read/
// Subscribe), so that suite can't run against it. Prune once it can.
package postgres_test

import (
	"errors"
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/crab-apple/dcb-event-store-pg-go/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTags(t *testing.T, values ...string) eventstore.Tags {
	t.Helper()
	tags, err := eventstore.TagsFrom(values)
	require.NoError(t, err)
	return tags
}

func mustQuery(t *testing.T, items ...eventstore.QueryItem) eventstore.Query {
	t.Helper()
	query, err := eventstore.QueryFromItems(items)
	require.NoError(t, err)
	return query
}

func newInstalledStore(t *testing.T) (*postgres.Store, *pgxpool.Pool) {
	t.Helper()
	pool := newTestPool(t)
	store, err := postgres.New(pool, postgres.Options{})
	require.NoError(t, err)
	require.NoError(t, store.EnsureInstalled(t.Context()))
	return store, pool
}

func TestAppend_InsertsAndReturnsPosition(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)

	// When
	pos, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{{Type: "A", Tags: mustTags(t, "tag-1")}},
	})

	// Then
	require.NoError(t, err)
	assert.Equal(t, "1", pos.String())
}

func TestAppend_SequentialAppendsIncreasePosition(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)

	// When
	pos1, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{{Type: "A"}},
	})
	require.NoError(t, err)
	pos2, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{{Type: "B"}},
	})
	require.NoError(t, err)

	// Then
	assert.True(t, pos2.IsAfter(pos1))
}

func TestAppend_RejectsZeroEvents(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)

	// When
	_, err := store.Append(t.Context(), eventstore.AppendCommand{})

	// Then
	assert.Error(t, err)
}

func TestAppend_RejectsInvalidCondition(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)

	// When
	_, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events:    []eventstore.Event{{Type: "A"}},
		Condition: &eventstore.AppendCondition{FailIfEventsMatch: eventstore.QueryAll()},
	})

	// Then
	require.Error(t, err)
	var conditionErr *eventstore.AppendConditionError
	assert.False(t, errors.As(err, &conditionErr))
}

func TestAppend_ConditionSucceedsWhenNothingMatches(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	condition := eventstore.AppendCondition{
		FailIfEventsMatch: mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "tag-1")}),
	}

	// When
	_, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events:    []eventstore.Event{{Type: "A", Tags: mustTags(t, "tag-1")}},
		Condition: &condition,
	})

	// Then
	assert.NoError(t, err)
}

func TestAppend_ConditionFailsWhenMatchExists(t *testing.T) {
	// Given
	store, _ := newInstalledStore(t)
	_, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{{Type: "A", Tags: mustTags(t, "tag-1")}},
	})
	require.NoError(t, err)

	// When
	condition := eventstore.AppendCondition{
		FailIfEventsMatch: mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "tag-1")}),
	}
	_, err = store.Append(t.Context(), eventstore.AppendCommand{
		Events:    []eventstore.Event{{Type: "A", Tags: mustTags(t, "tag-1")}},
		Condition: &condition,
	})

	// Then
	var conditionErr *eventstore.AppendConditionError
	require.ErrorAs(t, err, &conditionErr)
	assert.Nil(t, conditionErr.CommandIndex)
	assert.Equal(t, condition, conditionErr.AppendCondition)
}

func TestAppend_MultiCommandAtomicity(t *testing.T) {
	// Given
	store, pool := newInstalledStore(t)
	_, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{{Type: "A", Tags: mustTags(t, "tag-1")}},
	})
	require.NoError(t, err)

	// When
	condition := eventstore.AppendCondition{
		FailIfEventsMatch: mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "tag-1")}),
	}
	_, err = store.Append(t.Context(),
		eventstore.AppendCommand{Events: []eventstore.Event{{Type: "B"}}},
		eventstore.AppendCommand{Events: []eventstore.Event{{Type: "C"}}, Condition: &condition},
	)

	// Then
	var conditionErr *eventstore.AppendConditionError
	require.ErrorAs(t, err, &conditionErr)
	require.NotNil(t, conditionErr.CommandIndex)
	assert.Equal(t, 1, *conditionErr.CommandIndex)

	var count int
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM events WHERE type IN ('B','C')`).Scan(&count))
	assert.Equal(t, 0, count)
}
