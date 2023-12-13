package gradle

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitGradleTasks(t *testing.T) {
	tests := []struct {
		name           string
		input          []string
		expectedOutput []string
	}{
		{
			name:           "Test with single task",
			input:          []string{"task1"},
			expectedOutput: []string{"task1"},
		},
		{
			name:           "Test with multiple tasks separated by spaces",
			input:          []string{"task1 task2 task3"},
			expectedOutput: []string{"task1", "task2", "task3"},
		},
		{
			name:           "Test with tasks containing double quotes",
			input:          []string{`task1 -Dproperty="value1 value2" task4`},
			expectedOutput: []string{"task1", `-Dproperty="value1 value2"`, "task4"},
		},
		{
			name:           "Test with tasks containing single quotes",
			input:          []string{`task1 -Dproperty='value1 value2' task4`},
			expectedOutput: []string{"task1", `-Dproperty='value1 value2'`, "task4"},
		},
		{
			name:           "Test with empty input",
			input:          []string{},
			expectedOutput: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := SplitGradleTasks(test.input...)
			assert.ElementsMatch(t, test.expectedOutput, result)
		})
	}
}
