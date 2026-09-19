package dcb

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Tags is an immutable, unordered set of tag values. Tags are the primary
// mechanism for scoping events in the DCB pattern
type Tags struct {
	values map[string]struct{}
}

// Values returns the tag values as a slice.
// The order of the tags within the slice is undefined. Not guaranteed to
// be deterministic or even consistent across calls.
func (t Tags) Values() []string {
	values := make([]string, 0, len(t.values))
	for v := range t.values {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

// Len returns the number of distinct tag values.
func (t Tags) Len() int {
	return len(t.values)
}

// Equal reports whether two Tags contain the same set of values.
func (t Tags) Equal(other Tags) bool {
	if len(t.values) != len(other.values) {
		return false
	}
	for v := range t.values {
		if _, ok := other.values[v]; !ok {
			return false
		}
	}
	return true
}

func isValidTagToken(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsSpace)
}

// TagsFrom creates Tags from a slice of raw string values. Each value must be
// non-empty and contain no whitespace.
func TagsFrom(values []string) (Tags, error) {
	for _, v := range values {
		if !isValidTagToken(v) {
			return Tags{}, fmt.Errorf("invalid tag value: %q", v)
		}
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return Tags{values: set}, nil
}

// TagsFromMap creates Tags from a key-value map. Keys and values must be
// non-empty and contain no whitespace. Each entry becomes a "key=value"
// string.
func TagsFromMap(obj map[string]string) (Tags, error) {
	if len(obj) == 0 {
		return Tags{}, fmt.Errorf("empty object is not valid for TagsFromMap")
	}
	set := make(map[string]struct{}, len(obj))
	for k, v := range obj {
		if !isValidTagToken(k) {
			return Tags{}, fmt.Errorf("invalid tag key: %q", k)
		}
		if !isValidTagToken(v) {
			return Tags{}, fmt.Errorf("invalid tag value: %q for key %q", v, k)
		}
		set[k+"="+v] = struct{}{}
	}
	return Tags{values: set}, nil
}

// EmptyTags returns a Tags instance with no values.
func EmptyTags() Tags {
	return Tags{}
}
