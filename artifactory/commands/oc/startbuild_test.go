package oc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateAllowedOptions(t *testing.T) {
	ocStartBuildCmd := NewOcStartBuildCommand()

	testCases := []struct {
		args  []string
		valid bool
	}{
		{[]string{}, true},
		{[]string{"-F"}, true},
		{[]string{"-w"}, false},
		{[]string{"--wait", "false"}, false},
		{[]string{"-F", "--template={{.name}}", "-o='json'"}, false},
		{[]string{"repo", "-o='json'"}, false},
		{[]string{"--output=json", "-F"}, false},
	}

	for _, testCase := range testCases {
		ocStartBuildCmd = ocStartBuildCmd.SetOcArgs(testCase.args)
		err := ocStartBuildCmd.validateAllowedOptions()
		assert.Equal(t, testCase.valid, err == nil, "Test args:", testCase.args)
	}
}
