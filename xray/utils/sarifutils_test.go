package utils

import (
	"github.com/jfrog/gofrog/datastructures"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

func getRunWithDummyResults(results ...*sarif.Result) *sarif.Run {
	run := sarif.NewRunWithInformationURI("", "")
	ids := datastructures.MakeSet[string]()
	for _, result := range results {
		if !ids.Exists(*result.RuleID) {
			run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarif.NewRule(*result.RuleID))
			ids.Add(*result.RuleID)
		}
	}
	return run.WithResults(results)
}

func getDummyPassingResult(ruleId string) *sarif.Result {
	kind := "pass"
	return &sarif.Result{
		Kind:   &kind,
		RuleID: &ruleId,
	}
}

func getDummyResultWithOneLocation(fileName string, startLine, startCol int, snippet, ruleId string, level string) *sarif.Result {
	return &sarif.Result{
		Locations: []*sarif.Location{
			{
				PhysicalLocation: &sarif.PhysicalLocation{
					ArtifactLocation: &sarif.ArtifactLocation{URI: &fileName},
					Region: &sarif.Region{
						StartLine:   &startLine,
						StartColumn: &startCol,
						Snippet:     &sarif.ArtifactContent{Text: &snippet}}},
			},
		},
		Level:  &level,
		RuleID: &ruleId,
	}
}
