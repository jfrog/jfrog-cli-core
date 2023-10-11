package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

type OutputFormat string

const (
	// OutputFormat values
	Table      OutputFormat = "table"
	Json       OutputFormat = "json"
	SimpleJson OutputFormat = "simple-json"
	Sarif      OutputFormat = "sarif"

	BaseDocumentationURL = "https://docs.jfrog-applications.jfrog.io/jfrog-security-features/"
)

const MissingCveScore = "0"
const maxPossibleCve = 10.0

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif)}

var CurationOutputFormats = []string{string(Table), string(Json)}

type ResultsWriter struct {
	// The scan results.
	results *ExtendedScanResults
	// SimpleJsonError  Errors to be added to output of the SimpleJson format.
	simpleJsonError []formats.SimpleJsonError
	// Format  The output format.
	format OutputFormat
	// IncludeVulnerabilities  If true, include all vulnerabilities as part of the output. Else, include violations only.
	includeVulnerabilities bool
	// IncludeLicenses  If true, also include license violations as part of the output.
	includeLicenses bool
	// IsMultipleRoots  multipleRoots is set to true, in case the given results array contains (or may contain) results of several projects (like in binary scan).
	isMultipleRoots bool
	// PrintExtended, If true, show extended results.
	printExtended bool
	// The scanType (binary,dependency)
	scanType services.ScanType
	// Messages - Option array of messages, to be displayed if the format is Table
	messages []string
}

func NewResultsWriter(extendedScanResults *ExtendedScanResults) *ResultsWriter {
	return &ResultsWriter{results: extendedScanResults}
}

func (rw *ResultsWriter) SetOutputFormat(format OutputFormat) *ResultsWriter {
	rw.format = format
	return rw
}

func (rw *ResultsWriter) SetScanType(scanType services.ScanType) *ResultsWriter {
	rw.scanType = scanType
	return rw
}

func (rw *ResultsWriter) SetSimpleJsonError(jsonErrors []formats.SimpleJsonError) *ResultsWriter {
	rw.simpleJsonError = jsonErrors
	return rw
}

func (rw *ResultsWriter) SetIncludeVulnerabilities(includeVulnerabilities bool) *ResultsWriter {
	rw.includeVulnerabilities = includeVulnerabilities
	return rw
}

func (rw *ResultsWriter) SetIncludeLicenses(licenses bool) *ResultsWriter {
	rw.includeLicenses = licenses
	return rw
}

func (rw *ResultsWriter) SetIsMultipleRootProject(isMultipleRootProject bool) *ResultsWriter {
	rw.isMultipleRoots = isMultipleRootProject
	return rw
}

func (rw *ResultsWriter) SetPrintExtendedTable(extendedTable bool) *ResultsWriter {
	rw.printExtended = extendedTable
	return rw
}

func (rw *ResultsWriter) SetExtraMessages(messages []string) *ResultsWriter {
	rw.messages = messages
	return rw

}

// PrintScanResults prints the scan results in the specified format.
// Note that errors are printed only with SimpleJson format.
func (rw *ResultsWriter) PrintScanResults() error {
	switch rw.format {
	case Table:
		return rw.printScanResultsTables()
	case SimpleJson:
		jsonTable, err := rw.convertScanToSimpleJson()
		if err != nil {
			return err
		}
		return PrintJson(jsonTable)
	case Json:
		return PrintJson(rw.results.getXrayScanResults())
	case Sarif:
		sarifReport, err := GenereateSarifReportFromResults(rw.results, rw.isMultipleRoots, rw.includeLicenses)
		if err != nil {
			return err
		}
		sarifFile, err := ConvertSarifReportToString(sarifReport)
		if err != nil {
			return err
		}
		log.Output(sarifFile)
	}
	return nil
}
func (rw *ResultsWriter) printScanResultsTables() (err error) {
	printMessages(rw.messages)
	violations, vulnerabilities, licenses := SplitScanResults(rw.results.getXrayScanResults())
	if len(rw.results.getXrayScanResults()) > 0 {
		var resultsPath string
		if resultsPath, err = writeJsonResults(rw.results); err != nil {
			return
		}
		printMessage(coreutils.PrintTitle("The full scan results are available here: ") + coreutils.PrintLink(resultsPath))
	}
	log.Output()
	if rw.includeVulnerabilities {
		err = PrintVulnerabilitiesTable(vulnerabilities, rw.results, rw.isMultipleRoots, rw.printExtended, rw.scanType)
	} else {
		err = PrintViolationsTable(violations, rw.results, rw.isMultipleRoots, rw.printExtended, rw.scanType)
	}
	if err != nil {
		return
	}
	if rw.includeLicenses {
		if err = PrintLicensesTable(licenses, rw.printExtended, rw.scanType); err != nil {
			return
		}
	}
	if err = PrintSecretsTable(rw.results.SecretsScanResults, rw.results.EntitledForJas); err != nil {
		return
	}
	if err = PrintIacTable(rw.results.IacScanResults, rw.results.EntitledForJas); err != nil {
		return
	}
	return PrintSastTable(rw.results.SastScanResults, rw.results.EntitledForJas)
}

func printMessages(messages []string) {
	if len(messages) > 0 {
		log.Output()
	}
	for _, m := range messages {
		printMessage(m)
	}
}

func printMessage(message string) {
	log.Output("💬" + message)
}

func GenereateSarifReportFromResults(extendedResults *ExtendedScanResults, isMultipleRoots, includeLicenses bool) (report *sarif.Report, err error) {
	report, err = NewReport()
	if err != nil {
		return
	}
	xrayRun, err := convertXrayResponsesToSarifRun(extendedResults, isMultipleRoots, includeLicenses)
	if err != nil {
		return
	}

	report.Runs = append(report.Runs, xrayRun)
	report.Runs = append(report.Runs, extendedResults.ApplicabilityScanResults...)
	report.Runs = append(report.Runs, extendedResults.IacScanResults...)
	report.Runs = append(report.Runs, extendedResults.SecretsScanResults...)
	report.Runs = append(report.Runs, extendedResults.SastScanResults...)

	return
}

func ConvertSarifReportToString(report *sarif.Report) (sarifStr string, err error) {
	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return clientUtils.IndentJson(out), nil
}

func convertXrayResponsesToSarifRun(extendedResults *ExtendedScanResults, isMultipleRoots, includeLicenses bool) (run *sarif.Run, err error) {
	xrayJson, err := convertXrayScanToSimpleJson(extendedResults, isMultipleRoots, includeLicenses, true)
	if err != nil {
		return
	}
	xrayRun := sarif.NewRunWithInformationURI("JFrog Xray SCA", BaseDocumentationURL+"sca")
	xrayRun.Tool.Driver.Version = &extendedResults.XrayVersion
	if len(xrayJson.Vulnerabilities) > 0 || len(xrayJson.SecurityViolations) > 0 {
		if err = extractXrayIssuesToSarifRun(xrayRun, xrayJson); err != nil {
			return
		}
	}
	run = xrayRun
	return
}

func extractXrayIssuesToSarifRun(run *sarif.Run, xrayJson formats.SimpleJsonResults) error {
	for _, vulnerability := range xrayJson.Vulnerabilities {
		if err := addXrayCveIssueToSarifRun(
			vulnerability.Cves,
			vulnerability.IssueId,
			vulnerability.Severity,
			vulnerability.Technology,
			vulnerability.Components,
			vulnerability.Applicable,
			vulnerability.ImpactedDependencyName,
			vulnerability.ImpactedDependencyVersion,
			vulnerability.Summary,
			vulnerability.FixedVersions,
			run,
		); err != nil {
			return err
		}
	}
	for _, violation := range xrayJson.SecurityViolations {
		if err := addXrayCveIssueToSarifRun(
			violation.Cves,
			violation.IssueId,
			violation.Severity,
			violation.Technology,
			violation.Components,
			violation.Applicable,
			violation.ImpactedDependencyName,
			violation.ImpactedDependencyVersion,
			violation.Summary,
			violation.FixedVersions,
			run,
		); err != nil {
			return err
		}
	}
	for _, license := range xrayJson.LicensesViolations {
		msg := getVulnerabilityOrViolationSarifHeadline(license.LicenseKey, license.ImpactedDependencyName, license.ImpactedDependencyVersion)
		if rule, isNewRule := addResultToSarifRun(license.LicenseKey, msg, license.Severity, nil, run); isNewRule {
			rule.WithDescription("License watch violations")
		}
	}
	return nil
}

func addXrayCveIssueToSarifRun(cves []formats.CveRow, issueId, severity string, tech coreutils.Technology, components []formats.ComponentRow, applicable, impactedDependencyName, impactedDependencyVersion, summary string, fixedVersions []string, run *sarif.Run) error {
	maxCveScore, err := findMaxCVEScore(cves)
	if err != nil {
		return err
	}
	cveId := GetIssueIdentifier(cves, issueId)
	msg := getVulnerabilityOrViolationSarifHeadline(impactedDependencyName, impactedDependencyVersion, cveId)
	location, err := getXrayIssueLocationIfValidExists(tech, run)
	if err != nil {
		return err
	}
	if rule, isNewRule := addResultToSarifRun(cveId, msg, severity, location, run); isNewRule {
		cveRuleProperties := sarif.NewPropertyBag()
		if maxCveScore != MissingCveScore {
			cveRuleProperties.Add("security-severity", maxCveScore)
		}
		rule.WithProperties(cveRuleProperties.Properties)
		formattedDirectDependencies, err := getDirectDependenciesFormatted(components)
		if err != nil {
			return err
		}
		markdownDescription := getSarifTableDescription(formattedDirectDependencies, maxCveScore, applicable, fixedVersions) + "\n"
		rule.WithHelp(&sarif.MultiformatMessageString{
			Text:     &summary,
			Markdown: &markdownDescription,
		})
	}
	return nil
}

func getDescriptorFullPath(tech coreutils.Technology, run *sarif.Run) (string, error) {
	descriptors := tech.GetPackageDescriptor()
	if len(descriptors) == 1 {
		// Generate the full path
		return GetFullLocationFileName(strings.TrimSpace(descriptors[0]), run.Invocations), nil
	}
	for _, descriptor := range descriptors {
		// If multiple options return first to match
		absolutePath := GetFullLocationFileName(strings.TrimSpace(descriptor), run.Invocations)
		if exists, err := fileutils.IsFileExists(absolutePath, false); err != nil {
			return "", err
		} else if exists {
			return absolutePath, nil
		}
	}
	return "", nil
}

// Get the descriptor location with the Xray issues if exists.
func getXrayIssueLocationIfValidExists(tech coreutils.Technology, run *sarif.Run) (location *sarif.Location, err error) {
	descriptorPath, err := getDescriptorFullPath(tech, run)
	if err != nil {
		return
	}
	if strings.TrimSpace(descriptorPath) == "" {
		return
	}
	return sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://" + descriptorPath))), nil
}

func addResultToSarifRun(issueId, msg, severity string, location *sarif.Location, run *sarif.Run) (rule *sarif.ReportingDescriptor, isNewRule bool) {
	if rule, _ = run.GetRuleById(issueId); rule == nil {
		isNewRule = true
		rule = run.AddRule(issueId)
	}
	if result := run.CreateResultForRule(issueId).WithMessage(sarif.NewTextMessage(msg)).WithLevel(ConvertToSarifLevel(severity)); location != nil {
		result.AddLocation(location)
	}
	return
}

func convertXrayScanToSimpleJson(extendedResults *ExtendedScanResults, isMultipleRoots, includeLicenses, simplifiedOutput bool) (formats.SimpleJsonResults, error) {
	violations, vulnerabilities, licenses := SplitScanResults(extendedResults.XrayResults)
	jsonTable := formats.SimpleJsonResults{}
	if len(vulnerabilities) > 0 {
		vulJsonTable, err := PrepareVulnerabilities(vulnerabilities, extendedResults, isMultipleRoots, simplifiedOutput)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Vulnerabilities = vulJsonTable
	}
	if len(violations) > 0 {
		secViolationsJsonTable, licViolationsJsonTable, opRiskViolationsJsonTable, err := PrepareViolations(violations, extendedResults, isMultipleRoots, simplifiedOutput)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.SecurityViolations = secViolationsJsonTable
		jsonTable.LicensesViolations = licViolationsJsonTable
		jsonTable.OperationalRiskViolations = opRiskViolationsJsonTable
	}
	if includeLicenses {
		licJsonTable, err := PrepareLicenses(licenses)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Licenses = licJsonTable
	}

	return jsonTable, nil
}

func (rw *ResultsWriter) convertScanToSimpleJson() (formats.SimpleJsonResults, error) {
	jsonTable, err := convertXrayScanToSimpleJson(rw.results, rw.isMultipleRoots, rw.includeLicenses, false)
	if err != nil {
		return formats.SimpleJsonResults{}, err
	}
	if len(rw.results.SecretsScanResults) > 0 {
		jsonTable.Secrets = PrepareSecrets(rw.results.SecretsScanResults)
	}
	if len(rw.results.IacScanResults) > 0 {
		jsonTable.Iacs = PrepareIacs(rw.results.IacScanResults)
	}
	if len(rw.results.SastScanResults) > 0 {
		jsonTable.Sast = PrepareSast(rw.results.SastScanResults)
	}
	jsonTable.Errors = rw.simpleJsonError

	return jsonTable, nil
}

func GetIssueIdentifier(cvesRow []formats.CveRow, issueId string) string {
	var identifier string
	if len(cvesRow) != 0 {
		var cvesBuilder strings.Builder
		for _, cve := range cvesRow {
			cvesBuilder.WriteString(cve.Id + ", ")
		}
		identifier = strings.TrimSuffix(cvesBuilder.String(), ", ")
	}
	if identifier == "" {
		identifier = issueId
	}

	return identifier
}

func getVulnerabilityOrViolationSarifHeadline(depName, version, key string) string {
	return fmt.Sprintf("[%s] %s %s", key, depName, version)
}

func getDirectDependenciesFormatted(directDependencies []formats.ComponentRow) (string, error) {
	var formattedDirectDependencies strings.Builder
	for _, dependency := range directDependencies {
		if _, err := formattedDirectDependencies.WriteString(fmt.Sprintf("`%s %s`<br/>", dependency.Name, dependency.Version)); err != nil {
			return "", err
		}
	}
	return strings.TrimSuffix(formattedDirectDependencies.String(), "<br/>"), nil
}

func getSarifTableDescription(formattedDirectDependencies, maxCveScore, applicable string, fixedVersions []string) string {
	descriptionFixVersions := "No fix available"
	if len(fixedVersions) > 0 {
		descriptionFixVersions = strings.Join(fixedVersions, ", ")
	}
	if applicable == NotScanned.String() {
		return fmt.Sprintf("| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| %s      | %s       | %s   |",
			maxCveScore, formattedDirectDependencies, descriptionFixVersions)
	}
	return fmt.Sprintf("| Severity Score | Contextual Analysis | Direct Dependencies | Fixed Versions     |\n|  :---:  |  :---:  |  :---:  |  :---:  |\n| %s      | %s       | %s       | %s   |",
		maxCveScore, applicable, formattedDirectDependencies, descriptionFixVersions)
}

func findMaxCVEScore(cves []formats.CveRow) (string, error) {
	maxCve := 0.0
	for _, cve := range cves {
		if cve.CvssV3 == "" {
			continue
		}
		floatCve, err := strconv.ParseFloat(cve.CvssV3, 32)
		if err != nil {
			return "", err
		}
		if floatCve > maxCve {
			maxCve = floatCve
		}
		// if found maximum possible cve score, no need to keep iterating
		if maxCve == maxPossibleCve {
			break
		}
	}
	strCve := fmt.Sprintf("%.1f", maxCve)

	return strCve, nil
}

// Splits scan responses into aggregated lists of violations, vulnerabilities and licenses.
func SplitScanResults(results []services.ScanResponse) ([]services.Violation, []services.Vulnerability, []services.License) {
	var violations []services.Violation
	var vulnerabilities []services.Vulnerability
	var licenses []services.License
	for _, result := range results {
		violations = append(violations, result.Violations...)
		vulnerabilities = append(vulnerabilities, result.Vulnerabilities...)
		licenses = append(licenses, result.Licenses...)
	}
	return violations, vulnerabilities, licenses
}

func writeJsonResults(results *ExtendedScanResults) (resultsPath string, err error) {
	out, err := fileutils.CreateTempFile()
	if errorutils.CheckError(err) != nil {
		return
	}
	defer func() {
		e := out.Close()
		if err == nil {
			err = e
		}
	}()
	bytesRes, err := json.Marshal(&results)
	if errorutils.CheckError(err) != nil {
		return
	}
	var content bytes.Buffer
	err = json.Indent(&content, bytesRes, "", "  ")
	if errorutils.CheckError(err) != nil {
		return
	}
	_, err = out.Write(content.Bytes())
	if errorutils.CheckError(err) != nil {
		return
	}
	resultsPath = out.Name()
	return
}

func PrintJson(output interface{}) error {
	results, err := json.Marshal(output)
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Output(clientUtils.IndentJson(results))
	return nil
}

func CheckIfFailBuild(results []services.ScanResponse) bool {
	for _, result := range results {
		for _, violation := range result.Violations {
			if violation.FailBuild {
				return true
			}
		}
	}
	return false
}

func IsEmptyScanResponse(results []services.ScanResponse) bool {
	for _, result := range results {
		if len(result.Violations) > 0 || len(result.Vulnerabilities) > 0 || len(result.Licenses) > 0 {
			return false
		}
	}
	return true
}

func NewFailBuildError() error {
	return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
}
