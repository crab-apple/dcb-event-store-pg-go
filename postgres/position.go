package postgres

import (
	"fmt"
	"strconv"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// positionToInt64 and int64ToPosition round-trip a SequencePosition through
// its decimal string form to Postgres's bigint, since the type exposes no
// arithmetic or numeric accessor. Every SequencePosition here either came
// from int64ToPosition or from SequencePositionInitial, so the string is
// always a valid canonical, non-negative decimal integer fitting in int64.

func positionToInt64(pos eventstore.SequencePosition) int64 {
	n, err := strconv.ParseInt(pos.String(), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("postgres: impossible non-numeric position %q", pos.String()))
	}
	return n
}

func int64ToPosition(n int64) eventstore.SequencePosition {
	pos, err := eventstore.ParseSequencePosition(strconv.FormatInt(n, 10))
	if err != nil {
		panic(fmt.Sprintf("postgres: impossible invalid position %d", n))
	}
	return pos
}
