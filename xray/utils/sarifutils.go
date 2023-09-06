package utils

import (
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

type SarifLevel string

const (
	Error   SarifLevel = "error"
	Warning SarifLevel = "warning"
	Info    SarifLevel = "info"
	Note    SarifLevel = "note"
	None    SarifLevel = "none"

	SeverityDefaultValue = "Medium"
)

var (
	// All other values (include default) mapped as 'Medium' severity
	levelToSeverity = map[SarifLevel]string{
		Error: "High",
		Note:  "Low",
		None:  "Unknown",
	}

	mapSeverityToScore = map[string]string{
		"":         "0.0",
		"unknown":  "0.0",
		"low":      "3.9",
		"medium":   "6.9",
		"high":     "8.9",
		"critical": "10",
	}
)

func NewReport() (*sarif.Report, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return report, nil
}

func ReadScanRunsFromFile(fileName string) (sarifRuns []*sarif.Run, err error) {
	report, err := sarif.Open(fileName)
	if errorutils.CheckError(err) != nil {
		return
	}
	sarifRuns = report.Runs
	return
}

func XrayResponsesToSarifRun(responses []services.ScanResponse) *sarif.Run {
	xrayRun := sarif.NewRunWithInformationURI("JFrog Xray sca scanner", "https://jfrog.com/xray/")

	for _, response := range responses {

		xrayRun.Tool.Driver.Rules = append(xrayRun.Tool.Driver.Rules, sarif.NewRule(response.ScanId).
			WithHelp(sarif.NewMarkdownMultiformatMessageString("")).
			WithProperties(sarif.Properties{"": ""}),
		)
	}

	return xrayRun
}

// If exists SourceCodeScanResult with the same location as the provided SarifResult, return it
// func GetResultIfExists(result *sarif.Result, workingDir string, results []*SourceCodeScanResult) *SourceCodeScanResult {
// 	file := ExtractRelativePath(GetResultFileName(result), workingDir)
// 	lineCol := GetResultStartLocationInFile(result)
// 	text := *result.Message.Text
// 	for _, result := range results {
// 		if result.File == file && result.LineColumn == lineCol && result.Text == text {
// 			return result
// 		}
// 	}
// 	return nil
// }

// func ConvertSarifResultToSourceCodeScanResult(result *sarif.Result, workingDir string) *formats.SourceCodeScanResult {
// 	file := ExtractRelativePath(GetResultFileName(result), workingDir)
// 	lineCol := GetResultStartLocationInFile(result)
// 	text := *result.Message.Text

// 	return &SourceCodeScanResult{
// 		Severity: GetResultSeverity(result),
// 		SourceCodeLocation: SourceCodeLocation{
// 			File:       file,
// 			LineColumn: lineCol,
// 			Text:       text,
// 		},
// 		Type: *result.RuleID,
// 	}
// }

// func GetResultCodeFlows(result *sarif.Result, workingDir string) (flows []*[]SourceCodeLocation) {
// 	if len(result.CodeFlows) == 0 {
// 		return
// 	}
// 	for _, codeFlow := range result.CodeFlows {
// 		if codeFlow == nil || len(codeFlow.ThreadFlows) == 0 {
// 			continue
// 		}
// 		flows = append(flows, extractThreadFlows(codeFlow.ThreadFlows, workingDir)...)
// 	}
// 	return
// }

// func extractThreadFlows(threadFlows []*sarif.ThreadFlow, workingDir string) (flows []*[]SourceCodeLocation) {
// 	for _, threadFlow := range threadFlows {
// 		if threadFlow == nil || len(threadFlow.Locations) == 0 {
// 			continue
// 		}
// 		flow := extractStackTraceLocations(threadFlow.Locations, workingDir)
// 		if len(flow) > 0 {
// 			flows = append(flows, &flow)
// 		}
// 	}
// 	return
// }

// func extractStackTraceLocations(locations []*sarif.ThreadFlowLocation, workingDir string) (flow []SourceCodeLocation) {
// 	for _, location := range locations {
// 		if location == nil {
// 			continue
// 		}
// 		flow = append(flow, SourceCodeLocation{
// 			File:       ExtractRelativePath(GetLocationFileName(location.Location), workingDir),
// 			LineColumn: GetStartLocationInFile(location.Location),
// 			Text:       GetLocationSnippet(location.Location),
// 		})
// 	}
// 	return
// }

func GetResultMsgText(result *sarif.Result) string {
	return *result.Message.Text
}

func GetLocationSnippet(location *sarif.Location) string {
	snippet := GetLocationSnippetPointer(location)
	if snippet == nil {
		return ""
	}
	return *snippet
}

func GetLocationSnippetPointer(location *sarif.Location) *string {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		return location.PhysicalLocation.Region.Snippet.Text
	}
	return nil
}

func SetLocationSnippet(location *sarif.Location, val string) {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		location.PhysicalLocation.Region.Snippet.Text = &val
	}
	return
}

func GetLocationFileName(location *sarif.Location) string {
	filePath := location.PhysicalLocation.ArtifactLocation.URI
	if filePath != nil {
		return *filePath
	}
	return ""
}

func SetLocationFileName(location *sarif.Location, val string) {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		location.PhysicalLocation.ArtifactLocation.URI = &val
	}
	return
}

func GetStartLocationInFile(location *sarif.Location) string {
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

func isVulnerabilityResult(result *sarif.Result) bool {
	return !(result.Kind != nil && *result.Kind == "pass")
}

func GetCveNameFromRuleId(sarifRuleId string) string {
	return strings.TrimPrefix(sarifRuleId, "applic_")
}
