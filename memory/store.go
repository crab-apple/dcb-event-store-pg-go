// Package memory implements an in-memory eventstore.Store, for tests and
// prototyping.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

var _ eventstore.Store = (*Store)(nil)

// Store is an in-memory eventstore.Store. All events live in a single slice
// guarded by a mutex; nothing is persisted.
type Store struct {
	mu        sync.Mutex
	events    []eventstore.SequencedEvent
	broadcast chan struct{}
}

// New returns a Store, optionally pre-seeded with initialEvents.
func New(initialEvents ...eventstore.SequencedEvent) *Store {
	return &Store{
		events:    append([]eventstore.SequencedEvent(nil), initialEvents...),
		broadcast: make(chan struct{}),
	}
}

func (s *Store) Append(ctx context.Context, commands ...eventstore.AppendCommand) (eventstore.SequencePosition, error) {
	if err := ctx.Err(); err != nil {
		return eventstore.SequencePosition{}, err
	}
	for _, cmd := range commands {
		if cmd.Condition != nil {
			if err := eventstore.ValidateAppendCondition(*cmd.Condition); err != nil {
				return eventstore.SequencePosition{}, err
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Conditions are checked against this pre-call state throughout, so
	// sibling commands in the same batch never see each other's events.
	existing := s.events
	counter := counterFrom(lastPosition(existing))

	multiCommand := len(commands) > 1
	var newEvents []eventstore.SequencedEvent
	for i, cmd := range commands {
		if cmd.Condition != nil && anyMatchAfter(existing, cmd.Condition.FailIfEventsMatch, cmd.Condition.After) {
			var commandIndex *int
			if multiCommand {
				commandIndex = &i
			}
			return eventstore.SequencePosition{}, &eventstore.AppendConditionError{
				AppendCondition: *cmd.Condition,
				CommandIndex:    commandIndex,
			}
		}
		for _, ev := range cmd.Events {
			counter++
			newEvents = append(newEvents, eventstore.SequencedEvent{Event: ev, Position: positionFromCounter(counter)})
		}
	}

	if len(newEvents) == 0 {
		return eventstore.SequencePosition{}, fmt.Errorf("cannot append zero events")
	}

	s.events = append(s.events, newEvents...)
	close(s.broadcast)
	s.broadcast = make(chan struct{})

	return newEvents[len(newEvents)-1].Position, nil
}

func (s *Store) Read(ctx context.Context, query eventstore.Query, opts eventstore.ReadOptions) (eventstore.EventIterator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	events := matchingEvents(s.events, query, opts)
	s.mu.Unlock()
	return &sliceIterator{events: events}, nil
}

func (s *Store) Subscribe(
	ctx context.Context,
	query eventstore.Query,
	opts eventstore.SubscribeOptions,
) (eventstore.EventIterator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	position := eventstore.SequencePositionInitial()
	if opts.After != nil {
		position = *opts.After
	}
	return &subscribeIterator{store: s, ctx: ctx, query: query, position: position}, nil
}
