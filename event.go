package dcb

import "encoding/json"

// Event is the store's unit of data. Data and Metadata are opaque.
type Event struct {
	Type     string
	Tags     Tags
	Data     json.RawMessage
	Metadata json.RawMessage
}

// SequencedEvent is an Event together with its position in the store.
type SequencedEvent struct {
	Event    Event
	Position SequencePosition
}
