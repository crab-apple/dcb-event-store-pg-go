package eventstore

import "iter"

// EventIterator streams SequencedEvent results from Store.Read or
// Store.Subscribe. Next advances the iterator and reports whether Event is
// valid; a Subscribe iterator blocks in Next until an event is available or
// ctx is canceled. Callers must call Close when done.
//
// An EventIterator is not safe for concurrent use. In particular, Close must
// not be called while a Next call from another goroutine may still be in
// flight: cancel the iterator's context and wait for Next to return false
// before calling Close.
type EventIterator interface {
	Next() bool
	Event() SequencedEvent
	Err() error
	Close() error
}

// Collect drains it into a slice and closes it. Only use this with a Read
// iterator -- a Subscribe iterator does not terminate on its own.
func Collect(it EventIterator) ([]SequencedEvent, error) {
	defer func() { _ = it.Close() }()
	var events []SequencedEvent
	for it.Next() {
		events = append(events, it.Event())
	}
	return events, it.Err()
}

// All adapts it to a range-over-func iterator, closing it once the loop ends
// (by exhaustion or an early return): for ev, err := range All(it).
func All(it EventIterator) iter.Seq2[SequencedEvent, error] {
	return func(yield func(SequencedEvent, error) bool) {
		defer func() { _ = it.Close() }()
		for it.Next() {
			if !yield(it.Event(), nil) {
				return
			}
		}
		if err := it.Err(); err != nil {
			yield(SequencedEvent{}, err)
		}
	}
}
