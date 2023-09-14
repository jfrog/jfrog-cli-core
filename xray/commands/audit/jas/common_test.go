package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExcludeSuppressResults(t *testing.T) {
	tests := []struct {
		name           string
		sarifResults   []*sarif.Result
		expectedOutput []*sarif.Result
	}{
		{
			sarifResults: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet1", "ruleId1", "level1"),
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet2", "ruleId2", "level2"),
			},
			expectedOutput: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet1", "ruleId1", "level1"),
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet2", "ruleId2", "level2"),
			},
		},
		{
			sarifResults: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet1", "ruleId1", "level1").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet2", "ruleId2", "level2"),
			},
			expectedOutput: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet2", "ruleId2", "level2"),
			},
		},
		{
			sarifResults: []*sarif.Result{
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet1", "ruleId1", "level1").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
				utils.GetDummyResultWithOneLocation("", 0, 0, "snippet2", "ruleId2", "level2").WithSuppression([]*sarif.Suppression{sarif.NewSuppression("")}),
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
		sarifRun   *sarif.Run
		expectedOutput []*sarif.ReportingDescriptor
	}{
		{
			sarifRun: utils.GetRunWithDummyResults(
				utils.GetDummyResultWithOneLocation("file1",0,0, "snippet", "rule1", "info"),
				utils.GetDummyResultWithOneLocation("file2",0,0, "snippet", "rule1", "info"),
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule2", "warning"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{
				sarif.NewRule("rule1").WithProperties(sarif.Properties{"security-severity":"6.9"}),
				sarif.NewRule("rule2").WithProperties(sarif.Properties{"security-severity":"6.9"}),
			},
		},
		{
			sarifRun: utils.GetRunWithDummyResults(
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule1", "none"),
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule2", "note"),
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule3", "info"),
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule4", "warning"),
				utils.GetDummyResultWithOneLocation("file",0,0, "snippet", "rule5", "error"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{
				sarif.NewRule("rule1").WithProperties(sarif.Properties{"security-severity":"0.0"}),
				sarif.NewRule("rule2").WithProperties(sarif.Properties{"security-severity":"3.9"}),
				sarif.NewRule("rule3").WithProperties(sarif.Properties{"security-severity":"6.9"}),
				sarif.NewRule("rule4").WithProperties(sarif.Properties{"security-severity":"6.9"}),
				sarif.NewRule("rule5").WithProperties(sarif.Properties{"security-severity":"8.9"}),
			},
		},
	}

	for _, test := range tests {
		addScoreToRunRules(test.sarifRun)
		assert.Equal(t, test.expectedOutput, test.sarifRun.Tool.Driver.Rules)
	}
}
