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

// subscribeChannel drains it in the background and returns a channel of the
// events it yields. The channel closes once it.Next() returns false (e.g.
// after the subscription's context is canceled).
func subscribeChannel(it eventstore.EventIterator) <-chan eventstore.SequencedEvent {
	ch := make(chan eventstore.SequencedEvent, 32)
	go func() {
		defer close(ch)
		for it.Next() {
			ch <- it.Event()
		}
	}()
	return ch
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
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		// Then
		assert.Equal(t, posA, mustReceive(t, events).Position)
		assert.Equal(t, posB, mustReceive(t, events).Position)
	})

	t.Run("delivers events appended after subscribe starts", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		pos := mustAppendEvent(t, store, "Live", nil)

		// Then
		assert.Equal(t, pos, mustReceive(t, events).Position)
	})

	t.Run("delivers historical and live events seamlessly", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA := mustAppendEvent(t, store, "Historical", nil)

		// When
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		posB := mustAppendEvent(t, store, "Live", nil)

		// Then
		assert.Equal(t, posA, mustReceive(t, events).Position)
		assert.Equal(t, posB, mustReceive(t, events).Position)
	})

	t.Run("after option skips earlier events", func(t *testing.T) {
		// Given
		store := newStore(t)
		posA := mustAppendEvent(t, store, "A", nil)

		// When
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{After: new(posA)})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		posB := mustAppendEvent(t, store, "B", nil)

		// Then
		assert.Equal(t, posB, mustReceive(t, events).Position)
	})

	t.Run("delivers only events matching the query", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		query := mustQuery(t, []string{"Target"}, nil)
		it, err := store.Subscribe(t.Context(), query, eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		mustAppendEvent(t, store, "Noise", nil)
		pos := mustAppendEvent(t, store, "Target", nil)

		// Then
		received := mustReceive(t, events)
		assert.Equal(t, "Target", received.Event.Type)
		assert.Equal(t, pos, received.Position)
	})

	t.Run("multiple concurrent subscriptions see the same events", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		it1, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it1.Close() }()
		it2, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it2.Close() }()
		events1 := subscribeChannel(it1)
		events2 := subscribeChannel(it2)

		pos := mustAppendEvent(t, store, "Shared", nil)

		// Then
		assert.Equal(t, pos, mustReceive(t, events1).Position)
		assert.Equal(t, pos, mustReceive(t, events2).Position)
	})

	t.Run("position ordering is sequential across live appends", func(t *testing.T) {
		// Given
		store := newStore(t)

		// When
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)

		var appended []eventstore.SequencePosition
		for i := 0; i < 5; i++ {
			appended = append(appended, mustAppendEvent(t, store, "E", nil))
		}

		// Then
		for _, want := range appended {
			assert.Equal(t, want, mustReceive(t, events).Position)
		}
	})

	t.Run("context cancellation stops the subscription cleanly", func(t *testing.T) {
		// Given
		store := newStore(t)
		ctx, cancel := context.WithCancel(t.Context())

		// When
		it, err := store.Subscribe(ctx, eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)
		defer func() { _ = it.Close() }()
		events := subscribeChannel(it)
		cancel()

		// Then
		assertClosed(t, events)
		assert.NoError(t, it.Err())
	})
}
