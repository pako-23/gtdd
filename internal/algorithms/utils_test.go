package algorithms

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRemoveItem(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		list     []string
		index    int
		expected []string
	}{
		{[]string{"1", "2", "3", "4", "5"}, 2, []string{"1", "2", "4", "5"}},
		{[]string{"1", "2", "3", "4", "5"}, 0, []string{"2", "3", "4", "5"}},
		{[]string{"1", "2", "3", "4", "5"}, 4, []string{"1", "2", "3", "4"}},
	}

	for _, test := range tests {
		got := remove(test.list, test.index)

		assert.Equal(t, len(got), len(test.expected))
		for i := range test.expected {
			assert.Equal(t, got[i], test.expected[i])
		}
	}
}
