package utils

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

type SarifLevel string

const (
	errorLevel   SarifLevel = "error"
	warningLevel SarifLevel = "warning"
	infoLevel    SarifLevel = "info"
	noteLevel    SarifLevel = "note"
	noneLevel    SarifLevel = "none"

	SeverityDefaultValue = "Medium"

	applicabilityRuleIdPrefix = "applic_"
)

var (
	// All other values (include default) mapped as 'Medium' severity
	levelToSeverity = map[SarifLevel]string{
		errorLevel: "High",
		noteLevel:  "Low",
		noneLevel:  "Unknown",
	}

	severityToLevel = map[string]SarifLevel{
		"critical": errorLevel,
		"high":     errorLevel,
		"medium":   warningLevel,
		"low":      noteLevel,
		"unknown":  noneLevel,
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
		err = fmt.Errorf("can't read valid Sarif run from " + fileName + ": " + err.Error())
		return
	}
	sarifRuns = report.Runs
	return
}

func AggregateMultipleRunsIntoSingle(runs []*sarif.Run, destination *sarif.Run) {
	if len(runs) == 0 {
		return
	}
	for _, run := range runs {
		if run == nil || len(run.Results) == 0 {
			continue
		}
		for _, rule := range GetRunRules(run) {
			if destination.Tool.Driver != nil {
				destination.Tool.Driver.AddRule(rule)
			}
		}
		for _, result := range run.Results {
			destination.AddResult(result)
		}
		for _, invocation := range run.Invocations {
			destination.AddInvocations(invocation)
		}
	}
}

func getRunInformationUri(run *sarif.Run) string {
	if run != nil && run.Tool.Driver != nil && run.Tool.Driver.InformationURI != nil {
		return *run.Tool.Driver.InformationURI
	}
	return ""
}

// Calculate new information that exists at the run and not at the source
func GetDiffFromRun(sources []*sarif.Run, targets []*sarif.Run) (runWithNewOnly *sarif.Run) {
	// Combine
	combinedSource := sarif.NewRunWithInformationURI(sources[0].Tool.Driver.Name, getRunInformationUri(sources[0])).WithInvocations([]*sarif.Invocation{})
	AggregateMultipleRunsIntoSingle(sources, combinedSource)
	if combinedSource == nil {
		return
	}
	if len(targets) == 0 {
		return combinedSource
	}
	combinedTarget := sarif.NewRunWithInformationURI(targets[0].Tool.Driver.Name, getRunInformationUri(targets[0])).WithInvocations([]*sarif.Invocation{})
	AggregateMultipleRunsIntoSingle(targets, combinedTarget)
	if combinedTarget == nil {
		return combinedSource
	}
	// Get diff
	runWithNewOnly = sarif.NewRun(combinedSource.Tool).WithInvocations(combinedSource.Invocations)
	for _, sourceResult := range combinedSource.Results {
		targetMatchingResults := GetResultsByRuleId(combinedTarget, *sourceResult.RuleID)
		if len(targetMatchingResults) == 0 {
			runWithNewOnly.AddResult(sourceResult)
			if rule, _ := combinedSource.GetRuleById(*sourceResult.RuleID); rule != nil {
				runWithNewOnly.Tool.Driver.AddRule(rule)
			}
			continue
		}
		for _, targetMatchingResult := range targetMatchingResults {
			if len(sourceResult.Locations) > len(targetMatchingResult.Locations) ||
				len(sourceResult.CodeFlows) > len(targetMatchingResult.CodeFlows) {
				runWithNewOnly.AddResult(sourceResult)
				if rule, _ := combinedSource.GetRuleById(*sourceResult.RuleID); rule != nil {
					runWithNewOnly.Tool.Driver.AddRule(rule)
				}
			}
		}
	}
	return
}

func FilterResultsByRuleIdAndMsgText(source []*sarif.Result, ruleId, msgText string) (results []*sarif.Result) {
	for _, result := range source {
		if ruleId == *result.RuleID && msgText == GetResultMsgText(result) {
			results = append(results, result)
		}
	}
	return
}

func GetLocationRelatedCodeFlowsFromResult(location *sarif.Location, result *sarif.Result) (codeFlows []*sarif.CodeFlow) {
	for _, codeFlow := range result.CodeFlows {
		for _, stackTrace := range codeFlow.ThreadFlows {
			// The threadFlow is reverse stack trace.
			// The last location is the location that it relates to.
			if isSameLocation(location, stackTrace.Locations[len(stackTrace.Locations)-1].Location) {
				codeFlows = append(codeFlows, codeFlow)
			}
		}
	}
	return
}

func isSameLocation(location *sarif.Location, other *sarif.Location) bool {
	if location == other {
		return true
	}
	return GetLocationFileName(location) == GetLocationFileName(other) &&
		GetLocationSnippet(location) == GetLocationSnippet(other) &&
		GetLocationStartLine(location) == GetLocationStartLine(other) &&
		GetLocationStartColumn(location) == GetLocationStartColumn(other) &&
		GetLocationEndLine(location) == GetLocationEndLine(other) &&
		GetLocationEndColumn(location) == GetLocationEndColumn(other)
}

func GetResultsLocationCount(runs ...*sarif.Run) (count int) {
	for _, run := range runs {
		for _, result := range run.Results {
			count += len(result.Locations)
		}
	}
	return
}

func GetLevelResultsLocationCount(run *sarif.Run, level SarifLevel) (count int) {
	for _, result := range run.Results {
		if level == SarifLevel(*result.Level) {
			count += len(result.Locations)
		}
	}
	return
}

func GetResultsByRuleId(run *sarif.Run, ruleId string) (results []*sarif.Result) {
	for _, result := range run.Results {
		if *result.RuleID == ruleId {
			results = append(results, result)
		}
	}
	return
}

func GetResultMsgText(result *sarif.Result) string {
	if result.Message.Text != nil {
		return *result.Message.Text
	}
	return ""
}

func GetLocationSnippet(location *sarif.Location) string {
	snippet := GetLocationSnippetPointer(location)
	if snippet == nil {
		return ""
	}
	return *snippet
}

func GetLocationSnippetPointer(location *sarif.Location) *string {
	region := getLocationRegion(location)
	if region != nil && region.Snippet != nil {
		return region.Snippet.Text
	}
	return nil
}

func SetLocationSnippet(location *sarif.Location, snippet string) {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		location.PhysicalLocation.Region.Snippet.Text = &snippet
	}
}

func GetLocationFileName(location *sarif.Location) string {
	filePath := location.PhysicalLocation.ArtifactLocation.URI
	if filePath != nil {
		return *filePath
	}
	return ""
}

func GetRelativeLocationFileName(location *sarif.Location, invocations []*sarif.Invocation) string {
	wd := ""
	if len(invocations) > 0 {
		wd = GetInvocationWorkingDirectory(invocations[0])
	}
	GetLocationFileName(location)
	filePath := GetLocationFileName(location)
	if filePath != "" {
		return ExtractRelativePath(filePath, wd)
	}
	return ""
}

func SetLocationFileName(location *sarif.Location, fileName string) {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		location.PhysicalLocation.ArtifactLocation.URI = &fileName
	}
}

func getLocationRegion(location *sarif.Location) *sarif.Region {
	if location != nil && location.PhysicalLocation != nil {
		return location.PhysicalLocation.Region
	}
	return nil
}

func GetLocationStartLine(location *sarif.Location) int {
	region := getLocationRegion(location)
	if region != nil && region.StartLine != nil {
		return *region.StartLine
	}
	return 0
}

func GetLocationStartColumn(location *sarif.Location) int {
	region := getLocationRegion(location)
	if region != nil && region.StartColumn != nil {
		return *region.StartColumn
	}
	return 0
}

func GetLocationEndLine(location *sarif.Location) int {
	region := getLocationRegion(location)
	if region != nil && region.EndLine != nil {
		return *region.EndLine
	}
	return 0
}

func GetLocationEndColumn(location *sarif.Location) int {
	region := getLocationRegion(location)
	if region != nil && region.EndColumn != nil {
		return *region.EndColumn
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
	// Remove OS-specific file prefix
	resultPath = strings.TrimPrefix(resultPath, "file:///private")
	resultPath = strings.TrimPrefix(resultPath, "file://")

	// Get relative path
	relativePath := strings.ReplaceAll(resultPath, projectRoot, "")
	return strings.TrimPrefix(relativePath, string(filepath.Separator))
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
	return string(noneLevel)
}

func IsApplicableResult(result *sarif.Result) bool {
	return !(result.Kind != nil && *result.Kind == "pass")
}

func GetRuleFullDescription(rule *sarif.ReportingDescriptor) string {
	if rule.FullDescription != nil && rule.FullDescription.Text != nil {
		return *rule.FullDescription.Text
	}
	return ""
}

func CveToApplicabilityRuleId(cveId string) string {
	return applicabilityRuleIdPrefix + cveId
}

func ApplicabilityRuleIdToCve(sarifRuleId string) string {
	return strings.TrimPrefix(sarifRuleId, applicabilityRuleIdPrefix)
}

func GetRunRules(run *sarif.Run) []*sarif.ReportingDescriptor {
	if run != nil && run.Tool.Driver != nil {
		return run.Tool.Driver.Rules
	}
	return []*sarif.ReportingDescriptor{}
}

func GetInvocationWorkingDirectory(invocation *sarif.Invocation) string {
	if invocation.WorkingDirectory != nil && invocation.WorkingDirectory.URI != nil {
		return *invocation.WorkingDirectory.URI
	}
	return ""
}
