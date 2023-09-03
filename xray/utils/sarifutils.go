package utils

import (
	"strconv"
	"strings"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// If exists SourceCodeScanResult with the same location as the provided SarifResult, return it
func GetResultIfExists(result *sarif.Result, workingDir string, results *[]SourceCodeScanResult) *SourceCodeScanResult {
	file := ExtractRelativePath(GetResultFileName(result), workingDir)
	lineCol := GetResultLocationInFile(result)
	text := *result.Message.Text
	for _, result := range *results {
		if result.File == file && result.LineColumn == lineCol && result.Text == text {
			return &result
		}
	}
	return nil
}

func ConvertSarifResultToSourceCodeScanResult(result *sarif.Result, workingDir string) *SourceCodeScanResult {
	file := ExtractRelativePath(GetResultFileName(result), workingDir)
	lineCol := GetResultLocationInFile(result)
	text := *result.Message.Text

	return &SourceCodeScanResult{
		Severity: GetResultSeverity(result),
		SourceCodeLocation: SourceCodeLocation{
			File:       file,
			LineColumn: lineCol,
			Text:       text,
		},
		Type: *result.RuleID,
	}
}

func GetResultCodeFlows(result *sarif.Result, workingDir string) (flows []*[]SourceCodeLocation) {
	if len(result.CodeFlows) == 0 {
		return
	}
	for _, codeFlow := range result.CodeFlows {
		if codeFlow == nil || len(codeFlow.ThreadFlows) == 0 {
			continue
		}
		flows = append(flows, extractThreadFlows(codeFlow.ThreadFlows, workingDir)...)
	}
	return
}

func extractThreadFlows(threadFlows []*sarif.ThreadFlow, workingDir string) (flows []*[]SourceCodeLocation) {
	for _, threadFlow := range threadFlows {
		if threadFlow == nil || len(threadFlow.Locations) == 0 {
			continue
		}
		flow := extractStackTraceLocations(threadFlow.Locations, workingDir)
		if len(*flow) > 0 {
			flows = append(flows, flow)
		}
	}
	return
}

func extractStackTraceLocations(locations []*sarif.ThreadFlowLocation, workingDir string) (flow *[]SourceCodeLocation) {
	for _, location := range locations {
		if location == nil {
			continue
		}
		*flow = append(*flow, SourceCodeLocation{
			File:       ExtractRelativePath(getResultFileName(location.Location), workingDir),
			LineColumn: getResultLocationInFile(location.Location),
			Text:       GetResultLocationSnippet(location.Location),
		})
	}
	return
}

func GetResultLocationSnippet(location *sarif.Location) string {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		return *location.PhysicalLocation.Region.Snippet.Text
	}
	return ""
}

func GetResultFileName(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return getResultFileName(result.Locations[0])
	}
	return ""
}

func getResultFileName(location *sarif.Location) string {
	filePath := location.PhysicalLocation.ArtifactLocation.URI
	if filePath != nil {
		return *filePath
	}
	return ""
}

func GetResultLocationInFile(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return getResultLocationInFile(result.Locations[0])
	}
	return ""
}

func getResultLocationInFile(location *sarif.Location) string {
	startLine := location.PhysicalLocation.Region.StartLine
	startColumn := location.PhysicalLocation.Region.StartColumn
	if startLine != nil && startColumn != nil {
		return strconv.Itoa(*startLine) + ":" + strconv.Itoa(*startColumn)
	}
	return ""
}

func ExtractRelativePath(resultPath string, projectRoot string) string {
	filePrefix := "file://"
	relativePath := strings.ReplaceAll(strings.ReplaceAll(resultPath, projectRoot, ""), filePrefix, "")
	return relativePath
}

func GetResultSeverity(result *sarif.Result) string {
	if result.Level != nil {
		if severity, ok := levelToSeverity[SarifLevel(strings.ToLower(*result.Level))]; ok {
			return severity
		}
	}
	return SeverityDefaultValue
}
