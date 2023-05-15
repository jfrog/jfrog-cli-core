package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRemoveDuplicateValues(t *testing.T) {
	tests := []struct {
		testedSlice    []string
		expectedResult []string
	}{
		{testedSlice: []string{"1", "1", "1", "3"}, expectedResult: []string{"1", "3"}},
		{testedSlice: []string{}, expectedResult: []string{}},
		{testedSlice: []string{"1", "2", "3", "4"}, expectedResult: []string{"1", "2", "3", "4"}},
		{testedSlice: []string{"1", "6", "1", "6", "2"}, expectedResult: []string{"1", "6", "2"}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, RemoveDuplicateValues(test.testedSlice))
	}
}
