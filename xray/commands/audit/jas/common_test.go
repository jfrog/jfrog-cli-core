package jas

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
)

func TestExcludeSuppressResults(t *testing.T) {
	tests := []struct {
		name           string
		sarifResults   []*sarif.Result
		expectedOutput []*sarif.Result
	}{
		{
			sarifResults: []*sarif.Result{
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet1", "ruleId1", "level1"),
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet2", "ruleId2", "level2"),
			},
			expectedOutput: []*sarif.Result{
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet1", "ruleId1", "level1"),
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet2", "ruleId2", "level2"),
			},
		},
		{
			sarifResults: []*sarif.Result{
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet1", "ruleId1", "level1").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet2", "ruleId2", "level2"),
			},
			expectedOutput: []*sarif.Result{
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet2", "ruleId2", "level2"),
			},
		},
		{
			sarifResults: []*sarif.Result{
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet1", "ruleId1", "level1").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
				utils.CreateDummyResultWithOneLocation("", 0, 0, 0, 0, "snippet2", "ruleId2", "level2").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
			},
			expectedOutput: []*sarif.Result{},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, excludeSuppressResults(test.sarifResults))
	}
}

func TestAddScoreToRunRules(t *testing.T) {

	tests := []struct {
		name           string
		sarifRun       *sarif.Run
		expectedOutput []*sarif.ReportingDescriptor
	}{
		{
			sarifRun: utils.CreateRunWithDummyResults(
				utils.CreateDummyResultWithOneLocation("file1", 0, 0, 0, 0, "snippet", "rule1", "info"),
				utils.CreateDummyResultWithOneLocation("file2", 0, 0, 0, 0, "snippet", "rule1", "info"),
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule2", "warning"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{
				sarif.NewRule("rule1").WithProperties(sarif.Properties{"security-severity": "6.9"}),
				sarif.NewRule("rule2").WithProperties(sarif.Properties{"security-severity": "6.9"}),
			},
		},
		{
			sarifRun: utils.CreateRunWithDummyResults(
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule1", "none"),
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule2", "note"),
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule3", "info"),
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule4", "warning"),
				utils.CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule5", "error"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{
				sarif.NewRule("rule1").WithProperties(sarif.Properties{"security-severity": "0.0"}),
				sarif.NewRule("rule2").WithProperties(sarif.Properties{"security-severity": "3.9"}),
				sarif.NewRule("rule3").WithProperties(sarif.Properties{"security-severity": "6.9"}),
				sarif.NewRule("rule4").WithProperties(sarif.Properties{"security-severity": "6.9"}),
				sarif.NewRule("rule5").WithProperties(sarif.Properties{"security-severity": "8.9"}),
			},
		},
	}

	for _, test := range tests {
		addScoreToRunRules(test.sarifRun)
		assert.Equal(t, test.expectedOutput, test.sarifRun.Tool.Driver.Rules)
	}
}
