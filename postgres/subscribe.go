package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultPollInterval = 100 * time.Millisecond

func (s *Store) Subscribe(
	ctx context.Context,
	query eventstore.Query,
	opts eventstore.SubscribeOptions,
) (eventstore.EventIterator, error) {
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	position := eventstore.SequencePositionInitial()
	if opts.After != nil {
		position = *opts.After
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, "LISTEN "+s.notifyChannel); err != nil {
		conn.Release()
		return nil, err
	}

	return &subscribeIterator{
		store:        s,
		ctx:          ctx,
		query:        query,
		position:     position,
		pollInterval: pollInterval,
		listenerConn: conn,
	}, nil
}

// subscribeIterator drains everything currently matching via Read, then
// blocks for either a pg_notify on the store's channel or pollInterval,
// whichever comes first, before checking again.
type subscribeIterator struct {
	store        *Store
	ctx          context.Context
	query        eventstore.Query
	position     eventstore.SequencePosition
	pollInterval time.Duration
	listenerConn *pgxpool.Conn

	pending []eventstore.SequencedEvent
	current eventstore.SequencedEvent
	err     error
	closed  bool
}

func (it *subscribeIterator) Next() bool {
	if it.err != nil || it.closed {
		return false
	}
	for {
		if len(it.pending) > 0 {
			it.current = it.pending[0]
			it.pending = it.pending[1:]
			it.position = it.current.Position
			return true
		}

		if it.ctx.Err() != nil {
			return false
		}

		events, err := it.drain()
		if err != nil {
			if it.ctx.Err() != nil {
				return false // ctx ended during drain; clean stop, not a real error
			}
			it.err = err
			return false
		}
		if len(events) > 0 {
			it.pending = events
			continue
		}

		waitCtx, cancel := context.WithTimeout(it.ctx, it.pollInterval)
		_, err = it.listenerConn.Conn().WaitForNotification(waitCtx)
		cancel()
		if err == nil {
			continue // notification arrived; re-check immediately
		}
		if it.ctx.Err() != nil {
			return false // subscription's own context ended; clean stop
		}
		if errors.Is(err, context.DeadlineExceeded) {
			continue // poll interval elapsed with no notification; re-check anyway
		}
		it.err = err
		return false
	}
}

func (it *subscribeIterator) drain() ([]eventstore.SequencedEvent, error) {
	after := it.position
	readIt, err := it.store.Read(it.ctx, it.query, eventstore.ReadOptions{After: &after})
	if err != nil {
		return nil, err
	}
	return eventstore.Collect(readIt)
}

func (it *subscribeIterator) Event() eventstore.SequencedEvent { return it.current }
func (it *subscribeIterator) Err() error                       { return it.err }

func (it *subscribeIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	cleanupCtx := context.Background()
	_, _ = it.listenerConn.Exec(cleanupCtx, "UNLISTEN "+it.store.notifyChannel)
	it.listenerConn.Release()
	return nil
}
