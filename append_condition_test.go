package dcb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAppendCondition_RejectsQueryAll(t *testing.T) {
	err := ValidateAppendCondition(AppendCondition{FailIfEventsMatch: QueryAll()})
	assert.Error(t, err)
}

func TestValidateAppendCondition_RejectsItemsMissingTypeOrTag(t *testing.T) {
	tags, err := TagsFromMap(map[string]string{"e": "1"})
	require.NoError(t, err)

	itemMissingTags, err := QueryFromItems([]QueryItem{{Types: []string{"A"}}})
	require.NoError(t, err)

	tests := []struct {
		name  string
		query Query
	}{
		{"item with no tags", itemMissingTags},
		// QueryFromItems already rejects a missing Types, so this can only be
		// reached via a raw literal -- constructible here since the test is
		// in package dcb.
		{"item with no types", Query{items: []QueryItem{{Tags: tags}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAppendCondition(AppendCondition{FailIfEventsMatch: tc.query})
			assert.Error(t, err)
		})
	}
}

func TestValidateAppendCondition_AcceptsScopedQuery(t *testing.T) {
	tags, err := TagsFromMap(map[string]string{"e": "1"})
	require.NoError(t, err)
	query, err := QueryFromItems([]QueryItem{{Types: []string{"A"}, Tags: tags}})
	require.NoError(t, err)

	assert.NoError(t, ValidateAppendCondition(AppendCondition{FailIfEventsMatch: query}))
}
