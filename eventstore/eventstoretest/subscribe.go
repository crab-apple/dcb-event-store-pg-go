package eventstoretest

import (
	"context"
	"testing"
	"time"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const subscribeTimeout = 2 * time.Second

// subscription drains a Subscribe iterator in the background. EventIterator
// is not safe for concurrent use, so stop (not a bare Close) is the only
// correct way to tear one down: it cancels first and waits for the draining
// goroutine to actually exit before calling Close.
type subscription struct {
	events <-chan eventstore.SequencedEvent
	cancel context.CancelFunc
	it     eventstore.EventIterator
}

func mustSubscribe(
	t *testing.T,
	store eventstore.Store,
	query eventstore.Query,
	opts eventstore.SubscribeOptions,
) *subscription {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	it, err := store.Subscribe(ctx, query, opts)
	require.NoError(t, err)

	ch := make(chan eventstore.SequencedEvent, 32)
	go func() {
		defer close(ch)
		for it.Next() {
			ch <- it.Event()
		}
	}()

	return &subscription{events: ch, cancel: cancel, it: it}
}

func (s *subscription) stop() {
	s.cancel()
	for range s.events {
		// Drain until the goroutine's Next loop observes the cancellation
		// and exits -- only then is it safe to Close.
	}
	_ = s.it.Close()
}

// mustReceive waits for the next event on events, failing the test if none
// arrives within subscribeTimeout.
func mustReceive(t *testing.T, events <-chan eventstore.SequencedEvent) eventstore.SequencedEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		require.True(t, ok, "subscription ended before yielding an event")
		return ev
	case <-time.After(subscribeTimeout):
		t.Fatal("timed out waiting for a subscribed event")
		panic("unreachable")
	}
}

// assertClosed waits for events to close, failing the test if it doesn't
// within subscribeTimeout.
func assertClosed(t *testing.T, events <-chan eventstore.SequencedEvent) {
	t.Helper()
	deadline := time.After(subscribeTimeout)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("subscription did not stop in time")
		}
	}
}

func testSubscribe(t *testing.T, newStore NewStore) {
	t.Run("delivers historical events first", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA := mustAppendEvent(t, store, "A", nil)
		posB := mustAppendEvent(t, store, "B", nil)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub.stop()

		// Then
		assert.Equal(t, posA, mustReceive(t, sub.events).Position)
		assert.Equal(t, posB, mustReceive(t, sub.events).Position)
	})

	t.Run("delivers events appended after subscribe starts", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub.stop()
		pos := mustAppendEvent(t, store, "Live", nil)

		// Then
		assert.Equal(t, pos, mustReceive(t, sub.events).Position)
	})

	t.Run("delivers historical and live events seamlessly", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA := mustAppendEvent(t, store, "Historical", nil)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub.stop()
		posB := mustAppendEvent(t, store, "Live", nil)

		// Then
		assert.Equal(t, posA, mustReceive(t, sub.events).Position)
		assert.Equal(t, posB, mustReceive(t, sub.events).Position)
	})

	t.Run("after option skips earlier events", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA := mustAppendEvent(t, store, "A", nil)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{After: new(posA)})
		defer sub.stop()
		posB := mustAppendEvent(t, store, "B", nil)

		// Then
		assert.Equal(t, posB, mustReceive(t, sub.events).Position)
	})

	t.Run("delivers only events matching the query", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		query := mustQuery(t, []string{"Target"}, nil)
		sub := mustSubscribe(t, store, query, eventstore.SubscribeOptions{})
		defer sub.stop()

		mustAppendEvent(t, store, "Noise", nil)
		pos := mustAppendEvent(t, store, "Target", nil)

		// Then
		received := mustReceive(t, sub.events)
		assert.Equal(t, "Target", received.Event.Type)
		assert.Equal(t, pos, received.Position)
	})

	t.Run("multiple concurrent subscriptions see the same events", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		sub1 := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub1.stop()
		sub2 := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub2.stop()

		pos := mustAppendEvent(t, store, "Shared", nil)

		// Then
		assert.Equal(t, pos, mustReceive(t, sub1.events).Position)
		assert.Equal(t, pos, mustReceive(t, sub2.events).Position)
	})

	t.Run("position ordering is sequential across live appends", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub.stop()

		var appended []eventstore.SequencePosition
		for i := 0; i < 5; i++ {
			appended = append(appended, mustAppendEvent(t, store, "E", nil))
		}

		// Then
		for _, want := range appended {
			assert.Equal(t, want, mustReceive(t, sub.events).Position)
		}
	})

	t.Run("context cancellation stops the subscription cleanly", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		sub := mustSubscribe(t, store, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		defer sub.stop()
		sub.cancel()

		// Then
		assertClosed(t, sub.events)
		assert.NoError(t, sub.it.Err())
	})
}
