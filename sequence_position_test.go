package dcb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequencePositionInitial(t *testing.T) {
	assert.Equal(t, "0", SequencePositionInitial().String())
}

func TestParseSequencePosition_AcceptsValid(t *testing.T) {
	tests := []string{"0", "1", "42", "18446744073709551615"}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			pos, err := ParseSequencePosition(s)
			require.NoError(t, err)
			assert.Equal(t, s, pos.String())
		})
	}
}

func TestParseSequencePosition_RejectsInvalid(t *testing.T) {
	tests := []string{"", "-1", "-0", "01", "1.5", "+1", "abc", " 1", "1 "}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			_, err := ParseSequencePosition(s)
			assert.Error(t, err)
		})
	}
}

func TestSequencePosition_Compare(t *testing.T) {
	mustParse := func(t *testing.T, s string) SequencePosition {
		t.Helper()
		pos, err := ParseSequencePosition(s)
		require.NoError(t, err)
		return pos
	}

	tests := []struct {
		name       string
		a, b       SequencePosition
		wantAfter  bool
		wantBefore bool
		wantEqual  bool
	}{
		{"a after b", mustParse(t, "5"), mustParse(t, "3"), true, false, false},
		{"a before b", mustParse(t, "3"), mustParse(t, "5"), false, true, false},
		{"a equal b", mustParse(t, "5"), mustParse(t, "5"), false, false, true},
		{"initial equal initial", SequencePositionInitial(), SequencePositionInitial(), false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantAfter, tc.a.IsAfter(tc.b))
			assert.Equal(t, tc.wantBefore, tc.a.IsBefore(tc.b))
			assert.Equal(t, tc.wantEqual, tc.a.Equal(tc.b))
		})
	}
}

func TestCompareSequencePosition(t *testing.T) {
	mustParse := func(t *testing.T, s string) SequencePosition {
		t.Helper()
		pos, err := ParseSequencePosition(s)
		require.NoError(t, err)
		return pos
	}

	tests := []struct {
		name string
		a, b SequencePosition
		want int
	}{
		{"a < b", mustParse(t, "3"), mustParse(t, "5"), -1},
		{"a > b", mustParse(t, "5"), mustParse(t, "3"), 1},
		{"a == b", mustParse(t, "5"), mustParse(t, "5"), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CompareSequencePosition(tc.a, tc.b))
		})
	}
}
