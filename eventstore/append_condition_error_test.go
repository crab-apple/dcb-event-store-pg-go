package eventstore

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendConditionError_Error(t *testing.T) {
	index := func(i int) *int { return &i }

	tests := []struct {
		name         string
		commandIndex *int
		want         string
	}{
		{"no command index", nil, "expected version fail: new events matching appendCondition found"},
		{"command index 0", index(0), "expected version fail: new events matching appendCondition found (command 0)"},
		{"command index 3", index(3), "expected version fail: new events matching appendCondition found (command 3)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &AppendConditionError{CommandIndex: tc.commandIndex}
			assert.Equal(t, tc.want, err.Error())
		})
	}
}

func TestAppendConditionError_DetectableViaErrorsAs(t *testing.T) {
	condition := AppendCondition{FailIfEventsMatch: QueryAll()}
	var err error = &AppendConditionError{AppendCondition: condition}

	var target *AppendConditionError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, condition, target.AppendCondition)
}

func TestAppendConditionError_NotMatchedByErrorsAsForOtherErrors(t *testing.T) {
	err := errors.New("some other error")

	var target *AppendConditionError
	assert.False(t, errors.As(err, &target))
}
