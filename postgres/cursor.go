package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const readBatchSize = 5000

type cursorIterator struct {
	ctx  context.Context
	conn *pgxpool.Conn

	buffer    []eventstore.SequencedEvent
	idx       int
	exhausted bool
	err       error
	closed    bool
}

func (it *cursorIterator) Next() bool {
	if it.err != nil || it.closed {
		return false
	}
	if it.idx < len(it.buffer) {
		it.idx++
		return true
	}
	if it.exhausted {
		return false
	}

	rows, err := it.conn.Query(it.ctx, fmt.Sprintf("FETCH %d FROM %q", readBatchSize, readCursorName))
	if err != nil {
		it.err = err
		return false
	}
	buffer, err := pgx.CollectRows(rows, rowToSequencedEvent)
	if err != nil {
		it.err = err
		return false
	}

	it.buffer = buffer
	it.idx = 1
	if len(buffer) < readBatchSize {
		it.exhausted = true
	}
	return len(buffer) > 0
}

func (it *cursorIterator) Event() eventstore.SequencedEvent { return it.buffer[it.idx-1] }
func (it *cursorIterator) Err() error                       { return it.err }

func (it *cursorIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	cleanupCtx := context.Background()
	_, _ = it.conn.Exec(cleanupCtx, fmt.Sprintf("CLOSE %q", readCursorName))
	_, _ = it.conn.Exec(cleanupCtx, "ROLLBACK")
	it.conn.Release()
	return nil
}

func rowToSequencedEvent(row pgx.CollectableRow) (eventstore.SequencedEvent, error) {
	var seqPos int64
	var eventType, payload string
	var tags []string
	if err := row.Scan(&seqPos, &eventType, &payload, &tags); err != nil {
		return eventstore.SequencedEvent{}, err
	}

	var decoded struct {
		Data     json.RawMessage `json:"data"`
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return eventstore.SequencedEvent{}, err
	}

	tagSet, err := eventstore.TagsFrom(tags)
	if err != nil {
		return eventstore.SequencedEvent{}, err
	}

	return eventstore.SequencedEvent{
		Event: eventstore.Event{
			Type:     eventType,
			Tags:     tagSet,
			Data:     decoded.Data,
			Metadata: decoded.Metadata,
		},
		Position: int64ToPosition(seqPos),
	}, nil
}
