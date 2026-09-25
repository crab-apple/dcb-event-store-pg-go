package postgres

import (
	"hash/fnv"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// Lock keys live in three disjoint namespaces, distinguished by prefix before
// hashing: leaf keys scope a writer mutex to one (type, tag) pair; type-intent
// and global-intent keys let a reader's barrier detect in-flight writers of a
// type, or of anything, without waiting on unrelated writers.

func leafKey(eventType, tag string) int64 {
	return hashKey("L:" + eventType + "|" + tag)
}

func typeIntentKey(eventType string) int64 {
	return hashKey("T:" + eventType)
}

var globalIntentKey = hashKey("G")

func hashKey(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

// writerLockKeys are what an append acquires: X on every leaf key (from its
// events and its condition), S on the type-intent key of each event's type
// plus the global-intent key.
type writerLockKeys struct {
	leafX   []int64
	intentS []int64
}

func computeWriterLockKeys(events []eventstore.Event, condition *eventstore.AppendCondition) writerLockKeys {
	leaf := map[int64]struct{}{}
	intent := map[int64]struct{}{}

	if condition != nil && !condition.FailIfEventsMatch.IsAll() {
		for _, item := range condition.FailIfEventsMatch.Items() {
			for _, t := range item.Types {
				for _, tag := range item.Tags.Values() {
					leaf[leafKey(t, tag)] = struct{}{}
				}
			}
		}
	}

	for _, ev := range events {
		intent[typeIntentKey(ev.Type)] = struct{}{}
		for _, tag := range ev.Tags.Values() {
			leaf[leafKey(ev.Type, tag)] = struct{}{}
		}
	}
	if len(events) > 0 {
		intent[globalIntentKey] = struct{}{}
	}

	return writerLockKeys{leafX: keysOf(leaf), intentS: keysOf(intent)}
}

// readerLockKeys are what a read barrier acquires: S on each leaf key for a
// (type, tag) filter, X on a type's intent key for a type-only filter, or X
// on the global-intent key for Query.All().
type readerLockKeys struct {
	leafS   []int64
	intentX []int64
}

func computeReaderLockKeys(query eventstore.Query) readerLockKeys {
	if query.IsAll() {
		return readerLockKeys{intentX: []int64{globalIntentKey}}
	}

	leaf := map[int64]struct{}{}
	intent := map[int64]struct{}{}

	for _, item := range query.Items() {
		tags := item.Tags.Values()
		if len(tags) > 0 {
			for _, t := range item.Types {
				for _, tag := range tags {
					leaf[leafKey(t, tag)] = struct{}{}
				}
			}
			continue
		}
		for _, t := range item.Types {
			intent[typeIntentKey(t)] = struct{}{}
		}
	}

	return readerLockKeys{leafS: keysOf(leaf), intentX: keysOf(intent)}
}

func keysOf(set map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return keys
}
