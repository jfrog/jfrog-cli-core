package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"testing"
)


func TestExcludeSuppressResults(t *testing.T) {
	tests := []struct {
		name string
		sarifResults []*sarif.Result
		expectedOutput []*sarif.Result
	}{
		{
			sarifResults: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("").WithSuppression(),
				utils.GetDummyResultWithOneLocation("").WithSuppression()
			},
			expectedOutput: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("").WithSuppression(),
				utils.GetDummyResultWithOneLocation("").WithSuppression()
			},
		},
		{secret: "", expectedOutput: "***"},
		{secret: "12", expectedOutput: "***"},
		{secret: "123", expectedOutput: "***"},
		{secret: "123456789", expectedOutput: "123************"},
		{secret: "3478hfnkjhvd848446gghgfh", expectedOutput: "347************"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, excludeSuppressResults(test.sarifResults))
	}
}

func TestAddScoreToRunRules(t *testing.T) {

}
