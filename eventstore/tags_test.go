package eventstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsFrom_AcceptsValidValues(t *testing.T) {
	tests := []struct {
		name string
		in   []string
	}{
		{"key=value strings", []string{"courseId=c1", "studentId=s1"}},
		{"without key=value format", []string{"my-tag", "another_tag"}},
		{"single-character tag", []string{"x"}},
		{"hyphens", []string{"course-id=c1", "student-id=s1"}},
		{"underscores", []string{"course_id"}},
		{"colons", []string{"course:123"}},
		{"slashes", []string{"user/admin"}},
		{"dots", []string{"com.example.tag"}},
		{"multiple equals signs", []string{"key==value"}},
		{"special characters", []string{"key=value!", "@tag", "#hashtag", "price=$100"}},
		{"mixed key=value and bare tags", []string{"courseId=c1", "active", "priority:high"}},
		{"numeric-only", []string{"12345"}},
		{"empty array", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tags, err := TagsFrom(tc.in)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.in, tags.Values())
		})
	}
}

func TestTagsFrom_AcceptsNil(t *testing.T) {
	tags, err := TagsFrom(nil)
	require.NoError(t, err)
	assert.Empty(t, tags.Values())
}

func TestTagsFrom_DeduplicatesValues(t *testing.T) {
	tags, err := TagsFrom([]string{"a", "a", "b"})
	require.NoError(t, err)
	assert.Equal(t, 2, tags.Len())
	assert.ElementsMatch(t, []string{"a", "b"}, tags.Values())
}

func TestTagsFrom_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		in   []string
	}{
		{"empty string tag", []string{""}},
		{"tag containing a space", []string{"has space"}},
		{"tag containing a tab", []string{"has\ttab"}},
		{"tag containing a newline", []string{"has\nnewline"}},
		{"whitespace-only tag", []string{"   "}},
		{"tag containing a non-breaking space", []string{"has nbsp"}},
		{"any tag in array invalid", []string{"valid-tag", "invalid tag", "also-valid"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := TagsFrom(tc.in)
			assert.Error(t, err)
		})
	}
}

func TestTagsFromMap_AcceptsValidValues(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want []string
	}{
		{"multi-entry object", map[string]string{"courseId": "c1", "studentId": "s1"}, []string{"courseId=c1", "studentId=s1"}},
		{"single-entry object", map[string]string{"courseId": "c1"}, []string{"courseId=c1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tags, err := TagsFromMap(tc.in)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, tags.Values())
		})
	}
}

func TestTagsFromMap_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
	}{
		{"empty key", map[string]string{"": "value"}},
		{"empty value", map[string]string{"key": ""}},
		{"key containing whitespace", map[string]string{"course id": "c1"}},
		{"value containing whitespace", map[string]string{"courseId": "c 1"}},
		{"empty object", map[string]string{}},
		{"nil map", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := TagsFromMap(tc.in)
			assert.Error(t, err)
		})
	}
}

func TestTagsCreateEmpty(t *testing.T) {
	tags := EmptyTags()
	assert.Equal(t, 0, tags.Len())
	assert.Empty(t, tags.Values())
}

func TestTagsLen(t *testing.T) {
	mustTagsFrom := func(t *testing.T, values []string) Tags {
		t.Helper()
		tags, err := TagsFrom(values)
		require.NoError(t, err)
		return tags
	}

	tests := []struct {
		name string
		tags Tags
		want int
	}{
		{"empty tags", EmptyTags(), 0},
		{"tags from an empty slice", mustTagsFrom(t, []string{}), 0},
		{"tags from a non-empty slice", mustTagsFrom(t, []string{"a", "b", "c"}), 3},
		{"tags with duplicate values", mustTagsFrom(t, []string{"a", "a", "b"}), 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.tags.Len())
		})
	}
}

func TestTagsEqual(t *testing.T) {
	mustTagsFromMap := func(t *testing.T, obj map[string]string) Tags {
		t.Helper()
		tags, err := TagsFromMap(obj)
		require.NoError(t, err)
		return tags
	}
	mustTagsFrom := func(t *testing.T, values []string) Tags {
		t.Helper()
		tags, err := TagsFrom(values)
		require.NoError(t, err)
		return tags
	}

	tests := []struct {
		name string
		a, b Tags
		want bool
	}{
		{
			"identical objects",
			mustTagsFromMap(t, map[string]string{"courseId": "c1"}),
			mustTagsFromMap(t, map[string]string{"courseId": "c1"}),
			true,
		},
		{"two empty tags", EmptyTags(), EmptyTags(), true},
		{
			"identical non-key-value tags",
			mustTagsFrom(t, []string{"active", "priority:high"}),
			mustTagsFrom(t, []string{"active", "priority:high"}),
			true,
		},
		{
			"same values in a different order",
			mustTagsFrom(t, []string{"courseId=c1", "studentId=s1"}),
			mustTagsFrom(t, []string{"studentId=s1", "courseId=c1"}),
			true,
		},
		{
			"duplicate values collapse to the same set",
			mustTagsFrom(t, []string{"a", "a"}),
			mustTagsFrom(t, []string{"a"}),
			true,
		},
		{
			"different lengths",
			mustTagsFrom(t, []string{"courseId=c1"}),
			mustTagsFrom(t, []string{"courseId=c1", "studentId=s1"}),
			false,
		},
		{
			"values differ",
			mustTagsFrom(t, []string{"courseId=c1"}),
			mustTagsFrom(t, []string{"courseId=c2"}),
			false,
		},
		{"empty vs non-empty", EmptyTags(), mustTagsFrom(t, []string{"a"}), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.a.Equal(tc.b))
		})
	}
}
