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

// mustAppendEvent appends a single event as a single command.
func mustAppendEvent(t *testing.T, store eventstore.Store, eventType string, tags []string) eventstore.SequencePosition {
	t.Helper()
	pos, err := store.Append(t.Context(), eventstore.AppendCommand{
		Events: []eventstore.Event{mustEvent(t, eventType, tags)},
	})
	require.NoError(t, err)
	return pos
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
