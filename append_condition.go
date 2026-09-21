package dcb

import "fmt"

// AppendCondition is the optimistic-concurrency check for an append: the
// store fails the append if any event matching FailIfEventsMatch exists after
// After.
type AppendCondition struct {
	FailIfEventsMatch Query
	After             *SequencePosition
}

// ValidateAppendCondition checks that condition is scoped enough to compute
// lock keys from: FailIfEventsMatch must not be QueryAll(), and every item in
// it must specify at least one type and one tag.
func ValidateAppendCondition(condition AppendCondition) error {
	if condition.FailIfEventsMatch.IsAll() {
		return fmt.Errorf("append condition requires a scoped query; QueryAll() is not supported")
	}
	for _, item := range condition.FailIfEventsMatch.Items() {
		if len(item.Types) == 0 || item.Tags.Len() == 0 {
			return fmt.Errorf("append condition requires every query item to specify at least one type and one tag")
		}
	}
	return nil
}
