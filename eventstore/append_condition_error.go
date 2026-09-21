package eventstore

import "fmt"

// AppendConditionError is returned by EventStore.Append when events matching
// AppendCondition.FailIfEventsMatch exist after AppendCondition.After.
type AppendConditionError struct {
	AppendCondition AppendCondition
	// CommandIndex is the zero-based index of the failed command, or nil if
	// Append received a single command.
	CommandIndex *int
}

func (e *AppendConditionError) Error() string {
	if e.CommandIndex == nil {
		return "expected version fail: new events matching appendCondition found"
	}
	return fmt.Sprintf("expected version fail: new events matching appendCondition found (command %d)", *e.CommandIndex)
}
