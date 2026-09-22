package memory

import (
	"context"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// sliceIterator is Read's iterator: a fixed, already-computed result set.
type sliceIterator struct {
	events []eventstore.SequencedEvent
	idx    int
}

func (it *sliceIterator) Next() bool {
	if it.idx >= len(it.events) {
		return false
	}
	it.idx++
	return true
}

func (it *sliceIterator) Event() eventstore.SequencedEvent { return it.events[it.idx-1] }
func (it *sliceIterator) Err() error                       { return nil }
func (it *sliceIterator) Close() error                     { return nil }

// subscribeIterator is Subscribe's iterator. Next drains everything currently
// matching, then blocks on the store's broadcast channel for more.
//
// Correctness depends on snapshotting matching events and the current
// broadcast channel under the same lock: Append also mutates events and
// rotates broadcast under that lock, so if this snapshot sees no new events,
// any append that could add one must still close the exact channel captured
// here -- there is no window in which an append is missed.
type subscribeIterator struct {
	store    *Store
	ctx      context.Context
	query    eventstore.Query
	position eventstore.SequencePosition
	pending  []eventstore.SequencedEvent
	current  eventstore.SequencedEvent
}

func (it *subscribeIterator) Next() bool {
	for {
		if len(it.pending) > 0 {
			it.current = it.pending[0]
			it.pending = it.pending[1:]
			it.position = it.current.Position
			return true
		}

		it.store.mu.Lock()
		events := matchingEvents(it.store.events, it.query, eventstore.ReadOptions{After: &it.position})
		waitCh := it.store.broadcast
		it.store.mu.Unlock()

		if len(events) > 0 {
			it.pending = events
			continue
		}

		select {
		case <-waitCh:
			continue
		case <-it.ctx.Done():
			return false
		}
	}
}

func (it *subscribeIterator) Event() eventstore.SequencedEvent { return it.current }
func (it *subscribeIterator) Err() error                       { return nil }
func (it *subscribeIterator) Close() error                     { return nil }
