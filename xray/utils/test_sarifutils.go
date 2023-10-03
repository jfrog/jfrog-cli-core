package utils

import "github.com/owenrumney/go-sarif/v2/sarif"

func CreateRunWithDummyResults(results ...*sarif.Result) *sarif.Run {
	run := sarif.NewRunWithInformationURI("", "")
	for _, result := range results {
		if result.RuleID != nil {
			run.AddRule(*result.RuleID)
		}
		run.AddResult(result)
	}
	return run
}

func CreateResultWithLocations(msg, ruleId, level string, locations ...*sarif.Location) *sarif.Result {
	return &sarif.Result{
		Message:   *sarif.NewTextMessage(msg),
		Locations: locations,
		Level:     &level,
		RuleID:    &ruleId,
	}
}

func CreateLocation(fileName string, startLine, startCol, endLine, endCol int, snippet string) *sarif.Location {
	return &sarif.Location{
		PhysicalLocation: &sarif.PhysicalLocation{
			ArtifactLocation: &sarif.ArtifactLocation{URI: &fileName},
			Region: &sarif.Region{
				StartLine:   &startLine,
				StartColumn: &startCol,
				EndLine:     &endLine,
				EndColumn:   &endCol,
				Snippet:     &sarif.ArtifactContent{Text: &snippet}}},
	}
}

func CreateDummyPassingResult(ruleId string) *sarif.Result {
	kind := "pass"
	return &sarif.Result{
		Kind:   &kind,
		RuleID: &ruleId,
	}
}

func CreateResultWithOneLocation(fileName string, startLine, startCol, endLine, endCol int, snippet, ruleId, level string) *sarif.Result {
	return CreateResultWithLocations("", ruleId, level, CreateLocation(fileName, startLine, startCol, endLine, endCol, snippet))
}

func CreateCodeFlow(threadFlows ...*sarif.ThreadFlow) *sarif.CodeFlow {
	flow := sarif.NewCodeFlow()
	for _, threadFlow := range threadFlows {
		flow.AddThreadFlow(threadFlow)
	}
	return flow
}

func CreateThreadFlow(locations ...*sarif.Location) *sarif.ThreadFlow {
	stackStrace := sarif.NewThreadFlow()
	for _, location := range locations {
		stackStrace.AddLocation(sarif.NewThreadFlowLocation().WithLocation(location))
	}
	return stackStrace
}
