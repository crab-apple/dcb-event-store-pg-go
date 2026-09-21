package eventstoretest

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/require"
)

func mustEvent(t *testing.T, eventType string, tags map[string]string) eventstore.Event {
	t.Helper()
	tagSet := eventstore.EmptyTags()
	if len(tags) > 0 {
		var err error
		tagSet, err = eventstore.TagsFromMap(tags)
		require.NoError(t, err)
	}
	return eventstore.Event{Type: eventType, Tags: tagSet}
}

func scopedCondition(
	t *testing.T,
	types []string,
	tags map[string]string,
	after *eventstore.SequencePosition,
) eventstore.AppendCondition {
	t.Helper()
	tagSet, err := eventstore.TagsFromMap(tags)
	require.NoError(t, err)
	query, err := eventstore.QueryFromItems([]eventstore.QueryItem{{Types: types, Tags: tagSet}})
	require.NoError(t, err)
	return eventstore.AppendCondition{FailIfEventsMatch: query, After: after}
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
