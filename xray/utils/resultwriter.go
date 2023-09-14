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
)

const MissingCveScore = "0"
const maxPossibleCve = 10.0

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif)}

var CurationOutputFormats = []string{string(Table), string(Json)}

// PrintScanResults prints the scan results in the specified format.
// Note that errors are printed only with SimpleJson format.
//
// results - The scan results.
// simpleJsonError - Errors to be added to output of the SimpleJson format.
// format - The output format.
// includeVulnerabilities - If trie, include all vulnerabilities as part of the output. Else, include violations only.
// includeLicenses - If true, also include license violations as part of the output.
// isMultipleRoots - multipleRoots is set to true, in case the given results array contains (or may contain) results of several projects (like in binary scan).
// printExtended -If true, show extended results.
// scan - If true, use an output layout suitable for `jf scan` or `jf docker scan` results. Otherwise, use a layout compatible for `jf audit` .
// messages - Option array of messages, to be displayed if the format is Table
func PrintScanResults(results *ExtendedScanResults, simpleJsonError []formats.SimpleJsonError, format OutputFormat, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended, isBinaryScan bool, messages []string) error {
	switch format {
	case Table:
		return printScanResultsTables(results, isBinaryScan, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended, messages)
	case SimpleJson:
		jsonTable, err := convertScanToSimpleJson(results, simpleJsonError, isMultipleRoots, includeLicenses, false)
		if err != nil {
			return err
		}
		return PrintJson(jsonTable)
	case Json:
		return PrintJson(results.getXrayScanResults())
	case Sarif:
		sarifFile, err := GenerateSarifContentFromResults(results, isMultipleRoots, includeLicenses, false)
		if err != nil {
			return err
		}
		log.Output(sarifFile)
	}
	return nil
}

func printScanResultsTables(results *ExtendedScanResults, isBinaryScan, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended bool, messages []string) (err error) {
	printMessages(messages)
	violations, vulnerabilities, licenses := SplitScanResults(results.getXrayScanResults())
	if len(results.getXrayScanResults()) > 0 {
		var resultsPath string
		if resultsPath, err = writeJsonResults(results); err != nil {
			return
		}
		printMessage(coreutils.PrintTitle("The full scan results are available here: ") + coreutils.PrintLink(resultsPath))
	}
	log.Output()
	if includeVulnerabilities {
		err = PrintVulnerabilitiesTable(vulnerabilities, results, isMultipleRoots, printExtended, isBinaryScan)
	} else {
		err = PrintViolationsTable(violations, results, isMultipleRoots, printExtended, isBinaryScan)
	}
	if err != nil {
		return
	}
	if includeLicenses {
		if err = PrintLicensesTable(licenses, printExtended, isBinaryScan); err != nil {
			return
		}
	}
	if err = PrintSecretsTable(results.SecretsScanResults, results.EntitledForJas); err != nil {
		return
	}
	if err = PrintIacTable(results.IacScanResults, results.EntitledForJas); err != nil {
		return
	}
	if !IsSastSupported() {
		return
	}
	return PrintSastTable(results.SastScanResults, results.EntitledForJas)
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

func GenerateSarifContentFromResults(extendedResults *ExtendedScanResults, isMultipleRoots, includeLicenses, markdownOutput bool) (sarifStr string, err error) {
	report, err := NewReport()
	if err != nil {
		return
	}
	xrayRun, err := convertXrayResponsesToSarifRun(extendedResults, isMultipleRoots, includeLicenses, markdownOutput)
	if err != nil {
		return
	}

	report.Runs = append(report.Runs, xrayRun)
	report.Runs = append(report.Runs, extendedResults.ApplicabilityScanResults...)
	report.Runs = append(report.Runs, extendedResults.IacScanResults...)
	report.Runs = append(report.Runs, extendedResults.SecretsScanResults...)
	report.Runs = append(report.Runs, extendedResults.SastScanResults...)

	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return clientUtils.IndentJson(out), nil
}

func convertXrayResponsesToSarifRun(extendedResults *ExtendedScanResults, isMultipleRoots, includeLicenses, markdownOutput bool) (run *sarif.Run, err error) {
	xrayJson, err := convertXrayScanToSimpleJson(extendedResults, isMultipleRoots, includeLicenses, true)
	if err != nil {
		return
	}
	xrayRun := sarif.NewRunWithInformationURI("JFrog Xray Sca", "https://jfrog.com/xray/")
	xrayRun.Tool.Driver.Version = &extendedResults.XrayVersion
	if len(xrayJson.Vulnerabilities) > 0 || len(xrayJson.SecurityViolations) > 0 {
		if err = extractXrayIssuesToSarifRun(xrayRun, xrayJson, markdownOutput); err != nil {
			return
		}
	}
	run = xrayRun
	return
}

func extractXrayIssuesToSarifRun(run *sarif.Run, xrayJson formats.SimpleJsonResults, markdownOutput bool) error {
	for _, vulnerability := range xrayJson.Vulnerabilities {
		if err := addXrayCveIssueToSarifRun(
			vulnerability.Cves,
			vulnerability.IssueId,
			vulnerability.Severity,
			vulnerability.Technology.GetPackageDescriptor(),
			vulnerability.Components,
			vulnerability.Applicable,
			vulnerability.ImpactedDependencyName,
			vulnerability.ImpactedDependencyVersion,
			vulnerability.Summary,
			vulnerability.FixedVersions,
			markdownOutput,
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
			violation.Technology.GetPackageDescriptor(),
			violation.Components,
			violation.Applicable,
			violation.ImpactedDependencyName,
			violation.ImpactedDependencyVersion,
			violation.Summary,
			violation.FixedVersions,
			markdownOutput,
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

func addXrayCveIssueToSarifRun(cves []formats.CveRow, issueId, severity, file string, components []formats.ComponentRow, applicable, impactedDependencyName, impactedDependencyVersion, summary string, fixedVersions []string, markdownOutput bool, run *sarif.Run) error {
	maxCveScore, err := findMaxCVEScore(cves)
	if err != nil {
		return err
	}
	cveId := GetIssueIdentifier(cves, issueId)
	msg := getVulnerabilityOrViolationSarifHeadline(impactedDependencyName, impactedDependencyVersion, cveId)
	location := sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri(file)))

	if rule, isNewRule := addResultToSarifRun(cveId, msg, severity, location, run); isNewRule {
		cveRuleProperties := sarif.NewPropertyBag()
		if maxCveScore != MissingCveScore {
			cveRuleProperties.Add("security-severity", maxCveScore)
		}
		rule.WithProperties(cveRuleProperties.Properties)
		if markdownOutput {
			formattedDirectDependencies, err := getDirectDependenciesFormatted(components)
			if err != nil {
				return err
			}
			markdownDescription := getSarifTableDescription(formattedDirectDependencies, maxCveScore, applicable, fixedVersions) + "\n"
			rule.WithMarkdownHelp(markdownDescription)
		} else {
			rule.WithDescription(summary)
		}
	}
	return nil
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

func convertScanToSimpleJson(extendedResults *ExtendedScanResults, errors []formats.SimpleJsonError, isMultipleRoots, includeLicenses, simplifiedOutput bool) (formats.SimpleJsonResults, error) {
	jsonTable, err := convertXrayScanToSimpleJson(extendedResults, isMultipleRoots, includeLicenses, simplifiedOutput)
	if err != nil {
		return formats.SimpleJsonResults{}, err
	}
	if len(extendedResults.SecretsScanResults) > 0 {
		jsonTable.Secrets = PrepareSecrets(extendedResults.SecretsScanResults)
	}
	if len(extendedResults.IacScanResults) > 0 {
		jsonTable.Iacs = PrepareIacs(extendedResults.IacScanResults)
	}
	if len(extendedResults.SastScanResults) > 0 {
		jsonTable.Sast = PrepareSast(extendedResults.SastScanResults)
	}
	jsonTable.Errors = errors

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
