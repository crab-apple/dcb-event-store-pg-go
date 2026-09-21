package eventstore

import (
	"context"
	"time"
)

// Store is the DCB event-store contract.
type Store interface {
	// Append writes one or more commands atomically, returning the position
	// of the last event written. It returns an *AppendConditionError if any
	// command's condition is violated.
	Append(ctx context.Context, commands ...AppendCommand) (SequencePosition, error)

	// Read returns events matching query. The returned EventIterator is
	// bounded: it terminates once matching events are exhausted.
	Read(ctx context.Context, query Query, opts ReadOptions) (EventIterator, error)

	// Subscribe returns events matching query, then keeps yielding new
	// matches until ctx is canceled. The returned EventIterator never
	// terminates on its own.
	Subscribe(ctx context.Context, query Query, opts SubscribeOptions) (EventIterator, error)
}

// ReadOptions controls how Store.Read filters and orders a read.
type ReadOptions struct {
	// Backwards yields events in descending position order (newest first).
	Backwards bool
	// After restricts the read to positions after this one (or before, if
	// Backwards). Nil means no restriction.
	After *SequencePosition
	// Limit caps the number of events yielded. Zero means no limit.
	Limit int
}

// SubscribeOptions controls Store.Subscribe.
type SubscribeOptions struct {
	// After restricts the subscription to positions after this one. Nil
	// starts from the beginning of the stream.
	After *SequencePosition
	// PollInterval is the fallback polling interval for implementations that
	// poll for new events. Zero selects the implementation's default.
	PollInterval time.Duration
}
