package eventstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryAll(t *testing.T) {
	query := QueryAll()
	assert.True(t, query.IsAll())
	assert.Empty(t, query.Items())
}

func TestQueryFromItems_AcceptsValidItems(t *testing.T) {
	tags, err := TagsFromMap(map[string]string{"courseId": "c1"})
	require.NoError(t, err)
	items := []QueryItem{
		{Types: []string{"click"}},
		{Types: []string{"hover"}, Tags: tags},
	}

	query, err := QueryFromItems(items)
	require.NoError(t, err)
	assert.False(t, query.IsAll())
	assert.Equal(t, items, query.Items())
}

func TestQueryFromItems_RejectsInvalidItems(t *testing.T) {
	tags, err := TagsFromMap(map[string]string{"e": "1"})
	require.NoError(t, err)

	tests := []struct {
		name  string
		items []QueryItem
	}{
		{"nil items", nil},
		{"empty items", []QueryItem{}},
		{"item with nil types (tag-only filter)", []QueryItem{{Tags: tags}}},
		{"item with empty types", []QueryItem{{Types: []string{}, Tags: tags}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := QueryFromItems(tc.items)
			assert.Error(t, err)
		})
	}
}

func TestQuery_ItemsOnAllQueryIsNil(t *testing.T) {
	assert.Nil(t, QueryAll().Items())
}
