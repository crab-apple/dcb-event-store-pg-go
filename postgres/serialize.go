package postgres

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// serializeCommands flattens commands into events_append's parameter shape:
// one entry per event, and one condition row per (type, tag) pair in each
// command's condition query, tagged with that command's index.
func serializeCommands(commands []eventstore.AppendCommand) (
	types, tags, payloads []string,
	condIdxs []int32,
	condTypes, condTags []string,
	condAfter []int64,
) {
	for i, cmd := range commands {
		for _, ev := range cmd.Events {
			types = append(types, ev.Type)
			tags = append(tags, strings.Join(ev.Tags.Values(), tagDelimiter))
			payloads = append(payloads, serializePayload(ev))
		}
		if cmd.Condition == nil {
			continue
		}
		after := int64(0)
		if cmd.Condition.After != nil {
			after = positionToInt64(*cmd.Condition.After)
		}
		for _, item := range cmd.Condition.FailIfEventsMatch.Items() {
			for _, t := range item.Types {
				condIdxs = append(condIdxs, int32(i))
				condTypes = append(condTypes, t)
				condTags = append(condTags, strings.Join(item.Tags.Values(), tagDelimiter))
				condAfter = append(condAfter, after)
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
