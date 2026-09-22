package eventstoretest

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/require"
)

func mustEvent(t *testing.T, eventType string, tags []string) eventstore.Event {
	t.Helper()
	tagSet, err := eventstore.TagsFrom(tags)
	require.NoError(t, err)
	return eventstore.Event{Type: eventType, Tags: tagSet}
}

func mustQuery(t *testing.T, types []string, tags []string) eventstore.Query {
	t.Helper()
	tagSet, err := eventstore.TagsFrom(tags)
	require.NoError(t, err)
	query, err := eventstore.QueryFromItems([]eventstore.QueryItem{{Types: types, Tags: tagSet}})
	require.NoError(t, err)
	return query
}

func scopedCondition(
	t *testing.T,
	types []string,
	tags []string,
	after *eventstore.SequencePosition,
) eventstore.AppendCondition {
	t.Helper()
	return eventstore.AppendCondition{FailIfEventsMatch: mustQuery(t, types, tags), After: after}
}

func mustRead(
	t *testing.T,
	store eventstore.Store,
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
