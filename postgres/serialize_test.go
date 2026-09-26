package postgres

import (
	"encoding/json"
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/stretchr/testify/assert"
)

func TestSerializePayload_NilDataAndMetadataBecomeJSONNull(t *testing.T) {
	got := serializePayload(eventstore.Event{Type: "A"})
	assert.JSONEq(t, `{"data":null,"metadata":null}`, got)
}

func TestSerializePayload_PreservesProvidedDataAndMetadata(t *testing.T) {
	ev := eventstore.Event{
		Type:     "A",
		Data:     json.RawMessage(`{"x":1}`),
		Metadata: json.RawMessage(`{"y":2}`),
	}

	got := serializePayload(ev)

	assert.JSONEq(t, `{"data":{"x":1},"metadata":{"y":2}}`, got)
}
