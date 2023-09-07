package utils

import "github.com/owenrumney/go-sarif/v2/sarif"

func getDummyRunWithOneResult(fileName string, startLine, startCol int, snippet, ruleId string, level SarifLevel) *sarif.Run {
	return &sarif.Run{
		Results: []*sarif.Result{
			getDummyResultWithOneLocation(fileName, startLine, startCol, snippet, ruleId, string(level)),
		},
	}
}

// func getDummyPassingResult(ruleId string) *sarif.Result {
// 	kind := "pass"
// 	return &sarif.Result{
// 		Kind:   &kind,
// 		RuleID: &ruleId,
// 	}
// }

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
