package postgres_test

import (
	"testing"
	"time"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/require"
)

func TestSubscribe_CloseIsSafeWhileNextLoopsInBackground(t *testing.T) {
	store, _ := newInstalledStore(t)

	const iterations = 50
	for i := range iterations {
		it, err := store.Subscribe(t.Context(), eventstore.QueryAll(), eventstore.SubscribeOptions{})
		require.NoError(t, err)

		done := make(chan any, 1) // recovered panic value, or nil on clean return
		go func() {
			defer func() { done <- recover() }()
			for it.Next() {
				_ = it.Event()
			}
		}()

		_ = it.Close()

		select {
		case r := <-done:
			if r != nil {
				t.Fatalf("iteration %d: Next() panicked when Close() ran concurrently: %v", i, r)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: Next()-loop goroutine did not finish after Close()", i)
		}
	}
}
