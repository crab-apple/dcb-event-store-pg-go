package eventstore

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIterator struct {
	events []SequencedEvent
	err    error
	idx    int
	closed bool
}

func (f *fakeIterator) Next() bool {
	if f.idx >= len(f.events) {
		return false
	}
	f.idx++
	return true
}

func (f *fakeIterator) Event() SequencedEvent { return f.events[f.idx-1] }
func (f *fakeIterator) Err() error            { return f.err }
func (f *fakeIterator) Close() error {
	f.closed = true
	return nil
}

func newFakeEvents(t *testing.T, n int) []SequencedEvent {
	t.Helper()
	events := make([]SequencedEvent, n)
	for i := range events {
		pos, err := ParseSequencePosition(strconv.Itoa(i + 1))
		require.NoError(t, err)
		events[i] = SequencedEvent{Event: Event{Type: strconv.Itoa(i)}, Position: pos}
	}
	return events
}

func TestCollect_ReturnsEventsAndCloses(t *testing.T) {
	events := newFakeEvents(t, 3)
	it := &fakeIterator{events: events}

	got, err := Collect(it)

	require.NoError(t, err)
	assert.Equal(t, events, got)
	assert.True(t, it.closed)
}

func TestCollect_PropagatesIteratorError(t *testing.T) {
	wantErr := errors.New("boom")
	it := &fakeIterator{err: wantErr}

	got, err := Collect(it)

	assert.Nil(t, got)
	assert.Equal(t, wantErr, err)
	assert.True(t, it.closed)
}

func TestAll_YieldsEventsAndCloses(t *testing.T) {
	events := newFakeEvents(t, 3)
	it := &fakeIterator{events: events}

	var got []SequencedEvent
	for ev, err := range All(it) {
		require.NoError(t, err)
		got = append(got, ev)
	}

	assert.Equal(t, events, got)
	assert.True(t, it.closed)
}

func TestAll_StopsEarlyAndCloses(t *testing.T) {
	it := &fakeIterator{events: newFakeEvents(t, 3)}

	var got []SequencedEvent
	for ev := range All(it) {
		got = append(got, ev)
		break
	}

	assert.Len(t, got, 1)
	assert.True(t, it.closed)
}

func TestAll_YieldsErrorAfterExhaustingEvents(t *testing.T) {
	wantErr := errors.New("boom")
	events := newFakeEvents(t, 1)
	it := &fakeIterator{events: events, err: wantErr}

	var got []SequencedEvent
	var gotErr error
	for ev, err := range All(it) {
		if err != nil {
			gotErr = err
			continue
		}
		got = append(got, ev)
	}

	assert.Equal(t, events, got)
	assert.Equal(t, wantErr, gotErr)
}
