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

	severityToLevel = map[string]SarifLevel{
		"critical": Error,
		"high" : Error,
		"medium": Warning,
		"low": Note,
		"Unknown": None,
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

func CombineRuns(runs []*sarif.Run, overrideToolName, overrideUrl string) *sarif.Run {
	combined := sarif.NewRunWithInformationURI(overrideToolName, overrideUrl)
	
	rules := map[string]*sarif.ReportingDescriptor{}

	for _, run := range runs {
		for _, rule := range run.Tool.Driver.Rules {
			rules[rule.ID] = rule
		}
		for _, result := range run.Results {
			combined.Results = append(combined.Results, result)
		}
	}
	combinedRules := []*sarif.ReportingDescriptor{}
	for _, rule := range rules {
		combinedRules = append(combinedRules, rule)
	}
	combined.Tool.Driver.WithRules(combinedRules)
	return combined
} 


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

func GetLocationStartLine(location *sarif.Location) int {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.StartLine != nil {
		return *location.PhysicalLocation.Region.StartLine
	}
	return 0
}

func GetLocationStartColumn(location *sarif.Location) int {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.StartColumn != nil {
		return *location.PhysicalLocation.Region.StartColumn
	}
	return 0
}

func GetLocationEndLine(location *sarif.Location) int {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.EndLine != nil {
		return *location.PhysicalLocation.Region.EndLine
	}
	return 0
}

func GetLocationEndColumn(location *sarif.Location) int {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.EndColumn != nil {
		return *location.PhysicalLocation.Region.EndColumn
	}
	return 0
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

func ConvertToSarifLevel(severity string) string {
	if level, ok := severityToLevel[strings.ToLower(severity)]; ok {
		return string(level)
	}
	return string(None)
}

func ConvertScoreToSeverity(severity string) SarifLevel {
	if level, ok := severityToLevel[strings.ToLower(severity)]; ok {
		return level
	}
	return None
}

func isVulnerabilityResult(result *sarif.Result) bool {
	return !(result.Kind != nil && *result.Kind == "pass")
}

func GetCveNameFromRuleId(sarifRuleId string) string {
	return strings.TrimPrefix(sarifRuleId, "applic_")
}
