package memory

import (
	"slices"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

func matchesQuery(query eventstore.Query, ev eventstore.Event) bool {
	if query.IsAll() {
		return true
	}
	for _, item := range query.Items() {
		if matchesQueryItem(item, ev) {
			return true
		}
	}
	return false
}

func matchesQueryItem(item eventstore.QueryItem, ev eventstore.Event) bool {
	return slices.Contains(item.Types, ev.Type) && isTagSubset(item.Tags, ev.Tags)
}

// isTagSubset reports whether every value in filter is also present in tags.
func isTagSubset(filter, tags eventstore.Tags) bool {
	for _, v := range filter.Values() {
		if !slices.Contains(tags.Values(), v) {
			return false
		}
	}
	return true
}

// anyMatchAfter reports whether any event in events matches query at a
// position strictly after after (or anywhere, if after is nil).
func anyMatchAfter(events []eventstore.SequencedEvent, query eventstore.Query, after *eventstore.SequencePosition) bool {
	for _, se := range events {
		if after != nil && !se.Position.IsAfter(*after) {
			continue
		}
		if matchesQuery(query, se.Event) {
			return true
		}
	}
	return false
}

// matchingEvents filters, orders, and limits events per query and opts.
// events must already be in ascending position order.
func matchingEvents(
	events []eventstore.SequencedEvent,
	query eventstore.Query,
	opts eventstore.ReadOptions,
) []eventstore.SequencedEvent {
	var result []eventstore.SequencedEvent
	for _, se := range events {
		if opts.After != nil {
			if opts.Backwards {
				if !se.Position.IsBefore(*opts.After) {
					continue
				}
			} else if !se.Position.IsAfter(*opts.After) {
				continue
			}
		}
		if !matchesQuery(query, se.Event) {
			continue
		}
		result = append(result, se)
	}
	if opts.Backwards {
		slices.Reverse(result)
	}
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}
	return result
}
