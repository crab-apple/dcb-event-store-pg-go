package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRead_BarrierWaitsForSlowerInFlightCommit: writer A allocates a
// position but stays uncommitted while writer B allocates a higher position
// and commits. A concurrent Query.All Read must wait for A, not return B
// without it.
//
// Run per append path, since each acquires the writer locks the barrier
// waits on with its own code.
func TestRead_BarrierWaitsForSlowerInFlightCommit(t *testing.T) {
	for _, heldOpen := range []heldOpenWriter{appendHeldOpenViaFunction, appendHeldOpenViaCopy} {
		t.Run(heldOpen.name, func(t *testing.T) {
			assertBarrierWaitsForSlowerCommit(t, heldOpen)
		})
	}
}

func assertBarrierWaitsForSlowerCommit(t *testing.T, heldOpen heldOpenWriter) {
	store, pool := NewInstalledStore(t)
	ctx := t.Context()

	tagsA, err := eventstore.TagsFrom([]string{"scope=x"})
	require.NoError(t, err)
	tagsB, err := eventstore.TagsFrom([]string{"scope=y"})
	require.NoError(t, err)
	eventA := eventstore.Event{Type: "A", Tags: tagsA}
	eventB := eventstore.Event{Type: "B", Tags: tagsB}

	connA, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()
	txA, err := connA.Begin(ctx)
	require.NoError(t, err)
	heldOpen.run(t, ctx, txA, store, eventA)

	_, err = store.Append(ctx, eventstore.AppendCommand{Events: []eventstore.Event{eventB}})
	require.NoError(t, err)

	type readResult struct {
		events []eventstore.SequencedEvent
		err    error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		it, err := store.Read(ctx, eventstore.QueryAll(), eventstore.ReadOptions{})
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		events, err := eventstore.Collect(it)
		resultCh <- readResult{events: events, err: err}
	}()

	select {
	case <-resultCh:
		t.Fatal("Read returned before the slower in-flight commit (A) completed")
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, txA.Commit(ctx))

	var result readResult
	select {
	case result = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not unblock after the slower commit completed")
	}
	require.NoError(t, result.err)

	require.Len(t, result.events, 2)
	assert.Equal(t, "A", result.events[0].Event.Type)
	assert.Equal(t, "B", result.events[1].Event.Type)
	assert.True(t, result.events[0].Position.IsBefore(result.events[1].Position))
}

// A heldOpenWriter takes an append's writer locks and allocates a position
// inside tx without committing, leaving both held until the caller commits.
type heldOpenWriter struct {
	name string
	run  func(t *testing.T, ctx context.Context, tx pgx.Tx, s *Store, event eventstore.Event)
}

var appendHeldOpenViaFunction = heldOpenWriter{
	name: "stored procedure path",
	run: func(t *testing.T, ctx context.Context, tx pgx.Tx, s *Store, event eventstore.Event) {
		t.Helper()
		commands := []eventstore.AppendCommand{{Events: []eventstore.Event{event}}}
		keys := computeWriterLockKeys([]eventstore.Event{event}, nil)
		types, tags, payloads := serializeEvents(commands)
		condIdxs, condTypes, condTags, condAfter := serializeConditions(commands)
		var pos int64
		err := tx.QueryRow(ctx,
			fmt.Sprintf(`SELECT %s($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.appendFunctionName),
			keys.leafX, keys.intentS, types, tags, payloads, condIdxs, condTypes, condTags, condAfter,
		).Scan(&pos)
		require.NoError(t, err)
	},
}

var appendHeldOpenViaCopy = heldOpenWriter{
	name: "copy path",
	run: func(t *testing.T, ctx context.Context, tx pgx.Tx, s *Store, event eventstore.Event) {
		t.Helper()
		keys := computeWriterLockKeys([]eventstore.Event{event}, nil)
		_, err := tx.Exec(ctx, acquireWriterLocksSQL, keys.leafX, keys.intentS)
		require.NoError(t, err)

		_, err = tx.CopyFrom(ctx,
			pgx.Identifier{s.tableName},
			[]string{"type", "tags", "payload"},
			pgx.CopyFromSlice(1, func(int) ([]any, error) {
				return []any{event.Type, event.Tags.Values(), serializePayload(event)}, nil
			}),
		)
		require.NoError(t, err)
	},
}
