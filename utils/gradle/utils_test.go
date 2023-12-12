package gradleutils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitGradleTasks(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "clean aP -Dkarate.options='asd asdas'",
			expected: []string{"clean", "aP", "-Dkarate.options='asd asdas'"},
		},
		{
			input:    `clean aP -Dkarate.options="asd asdas"`,
			expected: []string{"clean", "aP", "-Dkarate.options=\"asd asdas\""},
		},
		{
			input:    "task1 'quoted argument' task2 -Dprop='value'",
			expected: []string{"task1", "'quoted argument'", "task2", "-Dprop='value'"},
		},
		{
			input:    "taskA -Dprop1=value1 -Dprop2='value 2'",
			expected: []string{"taskA", "-Dprop1=value1", "-Dprop2='value 2'"},
		},
	}

	for _, test := range tests {
		result := splitGradleTasks(test.input)
		assert.Equal(t, test.expected, result)
	}
}
