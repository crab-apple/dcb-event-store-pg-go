package postgres

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
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

func TestHashKey_IsDeterministic(t *testing.T) {
	assert.Equal(t, hashKey("A"), hashKey("A"))
}

func TestLeafKey_HashesPrefixedString(t *testing.T) {
	assert.Equal(t, hashKey("L:A|tag"), leafKey("A", "tag"))
}

func TestTypeIntentKey_HashesPrefixedString(t *testing.T) {
	assert.Equal(t, hashKey("T:A"), typeIntentKey("A"))
}

func TestGlobalIntentKey_HashesG(t *testing.T) {
	assert.Equal(t, hashKey("G"), globalIntentKey)
}

func TestComputeWriterLockKeys_FromEventsOnly(t *testing.T) {
	events := []eventstore.Event{{Type: "A", Tags: mustTags(t, "x", "y")}}

	keys := computeWriterLockKeys(events, nil)

	assert.ElementsMatch(t, []int64{leafKey("A", "x"), leafKey("A", "y")}, keys.leafX)
	assert.ElementsMatch(t, []int64{typeIntentKey("A"), globalIntentKey}, keys.intentS)
}

func TestComputeWriterLockKeys_FromConditionOnly(t *testing.T) {
	condition := &eventstore.AppendCondition{
		FailIfEventsMatch: mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "x")}),
	}

	keys := computeWriterLockKeys(nil, condition)

	assert.ElementsMatch(t, []int64{leafKey("A", "x")}, keys.leafX)
	assert.Empty(t, keys.intentS)
}

func TestComputeWriterLockKeys_DeduplicatesOverlappingLeafKeys(t *testing.T) {
	events := []eventstore.Event{{Type: "A", Tags: mustTags(t, "x")}}
	condition := &eventstore.AppendCondition{
		FailIfEventsMatch: mustQuery(t, eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "x")}),
	}

	keys := computeWriterLockKeys(events, condition)

	assert.Len(t, keys.leafX, 1)
}

func TestComputeWriterLockKeys_IgnoresAllQueryCondition(t *testing.T) {
	events := []eventstore.Event{{Type: "A", Tags: mustTags(t, "x")}}
	condition := &eventstore.AppendCondition{FailIfEventsMatch: eventstore.QueryAll()}

	keys := computeWriterLockKeys(events, condition)

	assert.ElementsMatch(t, []int64{leafKey("A", "x")}, keys.leafX)
}

func TestComputeReaderLockKeys_AllQuery(t *testing.T) {
	keys := computeReaderLockKeys(eventstore.QueryAll())

	assert.Empty(t, keys.leafS)
	assert.Equal(t, []int64{globalIntentKey}, keys.intentX)
}

func TestComputeReaderLockKeys_TypeAndTagFilter(t *testing.T) {
	query := mustQuery(t, eventstore.QueryItem{Types: []string{"A", "B"}, Tags: mustTags(t, "x")})

	keys := computeReaderLockKeys(query)

	assert.ElementsMatch(t, []int64{leafKey("A", "x"), leafKey("B", "x")}, keys.leafS)
	assert.Empty(t, keys.intentX)
}

func TestComputeReaderLockKeys_TypeOnlyFilter(t *testing.T) {
	query := mustQuery(t, eventstore.QueryItem{Types: []string{"A", "B"}})

	keys := computeReaderLockKeys(query)

	assert.Empty(t, keys.leafS)
	assert.ElementsMatch(t, []int64{typeIntentKey("A"), typeIntentKey("B")}, keys.intentX)
}

func TestComputeReaderLockKeys_MixedItems(t *testing.T) {
	query := mustQuery(t,
		eventstore.QueryItem{Types: []string{"A"}, Tags: mustTags(t, "x")},
		eventstore.QueryItem{Types: []string{"B"}},
	)

	keys := computeReaderLockKeys(query)

	assert.ElementsMatch(t, []int64{leafKey("A", "x")}, keys.leafS)
	assert.ElementsMatch(t, []int64{typeIntentKey("B")}, keys.intentX)
}
