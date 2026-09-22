package eventstoretest

import (
	"fmt"
	"sync"
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
)

// concurrently runs fn(0), fn(1), ..., fn(n-1) in separate goroutines,
// released together, and waits for all of them to return. fn must not call
// any testing.T method that stops the goroutine (t.Fatal, require.*, t.Helper
// callees using them) -- those are only safe from the test's own goroutine.
func concurrently(n int, fn func(i int)) {
	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	done.Add(n)
	for i := range n {
		go func() {
			defer done.Done()
			start.Wait()
			fn(i)
		}()
	}
	start.Done()
	done.Wait()
}

func allDistinct(positions []eventstore.SequencePosition) bool {
	seen := make(map[string]struct{}, len(positions))
	for _, pos := range positions {
		seen[pos.String()] = struct{}{}
	}
	return len(seen) == len(positions)
}

func testConcurrency(t *testing.T, newStore NewStore) {
	const n = 20

	t.Run("racing appends under the same condition: exactly one succeeds", func(t *testing.T) {
		// Given
		store := newStore(t)
		ctx := t.Context()
		event := mustEvent(t, "A", []string{"race"})
		condition := scopedCondition(t, []string{"A"}, []string{"race"}, nil)

		// When
		errs := make([]error, n)
		concurrently(n, func(i int) {
			_, err := store.Append(ctx, eventstore.AppendCommand{
				Events:    []eventstore.Event{event},
				Condition: &condition,
			})
			errs[i] = err
		})

		// Then
		successes, failures := 0, 0
		for _, err := range errs {
			if err == nil {
				successes++
				continue
			}
			failures++
			var conditionErr *eventstore.AppendConditionError
			assert.ErrorAs(t, err, &conditionErr)
		}
		assert.Equal(t, 1, successes)
		assert.Equal(t, n-1, failures)

		events := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})
		assert.Len(t, events, 1)
	})

	t.Run("concurrent appends to disjoint scopes all succeed", func(t *testing.T) {
		// Given
		store := newStore(t)
		ctx := t.Context()
		events := make([]eventstore.Event, n)
		conditions := make([]eventstore.AppendCondition, n)
		for i := range n {
			tag := fmt.Sprintf("tag-%d", i)
			events[i] = mustEvent(t, "A", []string{tag})
			conditions[i] = scopedCondition(t, []string{"A"}, []string{tag}, nil)
		}

		// When
		positions := make([]eventstore.SequencePosition, n)
		errs := make([]error, n)
		concurrently(n, func(i int) {
			pos, err := store.Append(ctx, eventstore.AppendCommand{
				Events:    []eventstore.Event{events[i]},
				Condition: &conditions[i],
			})
			positions[i] = pos
			errs[i] = err
		})

		// Then
		for _, err := range errs {
			assert.NoError(t, err)
		}
		assert.True(t, allDistinct(positions))

		readEvents := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})
		assert.Len(t, readEvents, n)
	})

	t.Run("concurrent unconditional appends produce no lost writes", func(t *testing.T) {
		// Given
		store := newStore(t)
		ctx := t.Context()
		events := make([]eventstore.Event, n)
		for i := range n {
			events[i] = mustEvent(t, "E", nil)
		}

		// When
		positions := make([]eventstore.SequencePosition, n)
		errs := make([]error, n)
		concurrently(n, func(i int) {
			pos, err := store.Append(ctx, eventstore.AppendCommand{Events: []eventstore.Event{events[i]}})
			positions[i] = pos
			errs[i] = err
		})

		// Then
		for _, err := range errs {
			assert.NoError(t, err)
		}
		assert.True(t, allDistinct(positions))

		readEvents := mustRead(t, store, eventstore.QueryAll(), eventstore.ReadOptions{})
		assert.Len(t, readEvents, n)
		assert.ElementsMatch(t, positions, positionsOf(readEvents))
	})
}
