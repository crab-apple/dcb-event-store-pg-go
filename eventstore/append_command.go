package eventstore

// AppendCommand is a unit of work for EventStore.Append: one or more events,
// with an optional AppendCondition checked before they're written.
type AppendCommand struct {
	Events    []Event
	Condition *AppendCondition
}
