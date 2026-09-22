package memory

import (
	"fmt"
	"strconv"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

func lastPosition(events []eventstore.SequencedEvent) eventstore.SequencePosition {
	if len(events) == 0 {
		return eventstore.SequencePositionInitial()
	}
	return events[len(events)-1].Position
}

func counterFrom(pos eventstore.SequencePosition) uint64 {
	n, err := strconv.ParseUint(pos.String(), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("memory: impossible non-numeric position %q", pos.String()))
	}
	return n
}

func positionFromCounter(n uint64) eventstore.SequencePosition {
	pos, err := eventstore.ParseSequencePosition(strconv.FormatUint(n, 10))
	if err != nil {
		panic(fmt.Sprintf("memory: impossible invalid counter %d", n))
	}
	return pos
}
