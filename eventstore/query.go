package eventstore

import "fmt"

// QueryItem is a single filter criterion within a Query: an event matches if
// its type is one of Types and its tags are a superset of Tags.
type QueryItem struct {
	Types []string
	Tags  Tags
}

// Query defines which events to read from the store: either every event, or
// the union of a non-empty set of QueryItem filters.
type Query struct {
	items []QueryItem
	all   bool
}

// QueryAll returns a Query matching every event. It cannot be used in an
// AppendCondition.
func QueryAll() Query {
	return Query{all: true}
}

// QueryFromItems returns a Query matching the union of the given items. Each
// item must have a non-empty Types; tag-only filters are not supported.
func QueryFromItems(items []QueryItem) (Query, error) {
	if len(items) == 0 {
		return Query{}, fmt.Errorf("query must be QueryAll() or a non-empty slice of QueryItem")
	}
	for i, item := range items {
		if len(item.Types) == 0 {
			return Query{}, fmt.Errorf(
				"query item %d must have a non-empty Types; tag-only filters are not supported, use QueryAll() to match every event",
				i,
			)
		}
	}
	copied := make([]QueryItem, len(items))
	copy(copied, items)
	return Query{items: copied}, nil
}

// IsAll reports whether q is the "match everything" query.
func (q Query) IsAll() bool {
	return q.all
}

// Items returns q's filter items, or nil if q.IsAll().
func (q Query) Items() []QueryItem {
	return q.items
}
