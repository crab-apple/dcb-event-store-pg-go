package dcb

import (
	"fmt"
	"strconv"
)

// SequencePosition is an opaque, comparable position in the global event
// stream. The zero value is the initial position (before any events).
type SequencePosition struct {
	value uint64
}

// IsAfter reports whether p is strictly after other.
func (p SequencePosition) IsAfter(other SequencePosition) bool {
	return p.value > other.value
}

// IsBefore reports whether p is strictly before other.
func (p SequencePosition) IsBefore(other SequencePosition) bool {
	return p.value < other.value
}

// Equal reports whether p and other are the same position.
func (p SequencePosition) Equal(other SequencePosition) bool {
	return p.value == other.value
}

// String returns p's string representation. Round-trips with
// ParseSequencePosition.
func (p SequencePosition) String() string {
	return strconv.FormatUint(p.value, 10)
}

// SequencePositionInitial returns the position before any events.
func SequencePositionInitial() SequencePosition {
	return SequencePosition{}
}

// ParseSequencePosition parses s as a non-negative integer. It rejects
// non-canonical forms such as "01", "-1", "1.5", and "+1".
func ParseSequencePosition(s string) (SequencePosition, error) {
	value, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != s {
		return SequencePosition{}, fmt.Errorf("invalid position string: %q", s)
	}
	return SequencePosition{value: value}, nil
}

// CompareSequencePosition returns -1, 0, or 1, for use as a sort comparator.
func CompareSequencePosition(a, b SequencePosition) int {
	switch {
	case a.value < b.value:
		return -1
	case a.value > b.value:
		return 1
	default:
		return 0
	}
}
