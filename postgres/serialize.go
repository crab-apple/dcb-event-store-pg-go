package postgres

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// serializeEvents flattens every command's events into events_append's
// per-event parameter shape.
func serializeEvents(commands []eventstore.AppendCommand) (types, tags, payloads []string) {
	for _, cmd := range commands {
		for _, ev := range cmd.Events {
			types = append(types, ev.Type)
			tags = append(tags, strings.Join(ev.Tags.Values(), tagDelimiter))
			payloads = append(payloads, serializePayload(ev))
		}
	}
	return
}

// serializeConditions flattens the commands' conditions into one row per
// (type, tag) pair, tagged with the index of the command it came from.
func serializeConditions(commands []eventstore.AppendCommand) (
	idxs []int32,
	types, tags []string,
	after []int64,
) {
	for i, cmd := range commands {
		if cmd.Condition == nil {
			continue
		}
		afterPos := int64(0)
		if cmd.Condition.After != nil {
			afterPos = positionToInt64(*cmd.Condition.After)
		}
		for _, item := range cmd.Condition.FailIfEventsMatch.Items() {
			for _, t := range item.Types {
				idxs = append(idxs, int32(i))
				types = append(types, t)
				tags = append(tags, strings.Join(item.Tags.Values(), tagDelimiter))
				after = append(after, afterPos)
			}
		}
	}
	return
}

func serializePayload(ev eventstore.Event) string {
	data, metadata := ev.Data, ev.Metadata
	if data == nil {
		data = json.RawMessage("null")
	}
	if metadata == nil {
		metadata = json.RawMessage("null")
	}
	return fmt.Sprintf(`{"data":%s,"metadata":%s}`, data, metadata)
}
