package scan

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

// The test only checks cases of returning an error in case of a violation with FailBuild == true
func TestPrintViolationsTable(t *testing.T) {
	tests := []struct {
		violations    []services.Violation
		expectedError bool
	}{
		{[]services.Violation{{FailBuild: false}, {FailBuild: false}, {FailBuild: false}}, false},
		{[]services.Violation{{FailBuild: false}, {FailBuild: true}, {FailBuild: false}}, true},
		{[]services.Violation{{FailBuild: true}, {FailBuild: true}, {FailBuild: true}}, true},
	}

	for _, test := range tests {
		err := PrintViolationsTable(test.violations)
		assert.Equal(t, test.expectedError, err != nil)
	}
}
