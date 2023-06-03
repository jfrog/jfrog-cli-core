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

const missingCveScore = "0"
const maxPossibleCve = 10.0

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif)}

var CurationOutputFormats = []string{string(Table), string(Json)}

type sarifProperties struct {
	Cves        string
	Headline    string
	Severity    string
	Description string
}

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
func PrintScanResults(results *ExtendedScanResults, simpleJsonError []formats.SimpleJsonError, format OutputFormat, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended, scan bool, messages []string) error {
	switch format {
	case Table:
		return printScanResultsTables(results, scan, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended, messages)
	case SimpleJson:
		jsonTable, err := convertScanToSimpleJson(results.getXrayScanResults(), results, simpleJsonError, isMultipleRoots, includeLicenses, false)
		if err != nil {
			return err
		}
		return PrintJson(jsonTable)
	case Json:
		return PrintJson(results.getXrayScanResults())
	case Sarif:
		sarifFile, err := GenerateSarifFileFromScan(results.getXrayScanResults(), results, isMultipleRoots, false)
		if err != nil {
			return err
		}
		log.Output(sarifFile)
	}
	return nil
}

func printScanResultsTables(results *ExtendedScanResults, scan, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended bool, messages []string) (err error) {
	log.Output()
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
		err = PrintVulnerabilitiesTable(vulnerabilities, results, isMultipleRoots, printExtended, scan)
	} else {
		err = PrintViolationsTable(violations, results, isMultipleRoots, printExtended, scan)
	}
	if err != nil {
		return
	}
	if includeLicenses {
		err = PrintLicensesTable(licenses, printExtended, scan)
	}
	return
}

func printMessages(messages []string) {
	for _, m := range messages {
		printMessage(m)
	}
}

func printMessage(message string) {
	log.Output("💬", message)
}

func GenerateSarifFileFromScan(currentScan []services.ScanResponse, extendedResults *ExtendedScanResults, isMultipleRoots, simplifiedOutput bool) (string, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	run := sarif.NewRunWithInformationURI("JFrog Xray", coreutils.JFrogComUrl+"xray/")
	if err = convertScanToSarif(run, currentScan, extendedResults, isMultipleRoots, simplifiedOutput); err != nil {
		return "", err
	}
	report.AddRun(run)
	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return clientUtils.IndentJson(out), nil
}

func convertScanToSimpleJson(results []services.ScanResponse, extendedResults *ExtendedScanResults, errors []formats.SimpleJsonError, isMultipleRoots, includeLicenses, simplifiedOutput bool) (formats.SimpleJsonResults, error) {
	violations, vulnerabilities, licenses := SplitScanResults(results)
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
	jsonTable.Errors = errors

	return jsonTable, nil
}

func convertScanToSarif(run *sarif.Run, currentScan []services.ScanResponse, extendedResults *ExtendedScanResults, isMultipleRoots, simplifiedOutput bool) error {
	var errors []formats.SimpleJsonError
	jsonTable, err := convertScanToSimpleJson(currentScan, extendedResults, errors, isMultipleRoots, false, simplifiedOutput)
	if err != nil {
		return err
	}
	if len(jsonTable.SecurityViolations) > 0 {
		return convertViolations(jsonTable, run, simplifiedOutput)
	}
	return convertVulnerabilities(jsonTable, run, simplifiedOutput)
}

func getCves(cvesRow []formats.CveRow, issueId string) string {
	var cvesStr string
	if len(cvesRow) != 0 {
		var cvesBuilder strings.Builder
		for _, cve := range cvesRow {
			cvesBuilder.WriteString(cve.Id + ", ")
		}
		cvesStr = strings.TrimSuffix(cvesBuilder.String(), ", ")
	}
	if cvesStr == "" {
		cvesStr = issueId
	}

	return cvesStr
}

func getHeadline(impactedPackage, version, key, fixVersion string) string {
	if fixVersion != "" {
		return fmt.Sprintf("[%s] Upgrade %s:%s to %s", key, impactedPackage, version, fixVersion)
	}
	return fmt.Sprintf("[%s] %s:%s", key, impactedPackage, version)
}

func convertViolations(jsonTable formats.SimpleJsonResults, run *sarif.Run, simplifiedOutput bool) error {
	for _, violation := range jsonTable.SecurityViolations {
		sarifProperties, err := getSarifProperties(violation, simplifiedOutput)
		if err != nil {
			return err
		}
		err = addScanResultsToSarifRun(run, sarifProperties.Severity, violation.IssueId, sarifProperties.Headline, sarifProperties.Description, violation.Technology)
		if err != nil {
			return err
		}
	}
	for _, license := range jsonTable.LicensesViolations {
		impactedPackageFull := getHeadline(license.ImpactedDependencyName, license.ImpactedDependencyVersion, license.LicenseKey, "")
		err := addScanResultsToSarifRun(run, "", license.ImpactedDependencyVersion, impactedPackageFull, license.LicenseKey, coreutils.Technology(strings.ToLower(license.ImpactedDependencyType)))
		if err != nil {
			return err
		}
	}

	return nil
}

func getSarifProperties(vulnerabilityRow formats.VulnerabilityOrViolationRow, simplifiedOutput bool) (sarifProperties, error) {
	cves := getCves(vulnerabilityRow.Cves, vulnerabilityRow.IssueId)
	fixVersion := getMinimalFixVersion(vulnerabilityRow.FixedVersions)
	headline := getHeadline(vulnerabilityRow.ImpactedDependencyName, vulnerabilityRow.ImpactedDependencyVersion, cves, fixVersion)
	maxCveScore, err := findMaxCVEScore(vulnerabilityRow.Cves)
	if err != nil {
		return sarifProperties{}, err
	}
	formattedDirectDependecies := getDirectDependenciesFormatted(vulnerabilityRow.Components)
	description := vulnerabilityRow.Summary
	if simplifiedOutput {
		description = getDescription(formattedDirectDependecies, maxCveScore, vulnerabilityRow.FixedVersions)
	}
	return sarifProperties{
		Cves:        cves,
		Headline:    headline,
		Severity:    maxCveScore,
		Description: description,
	}, err
}

func convertVulnerabilities(jsonTable formats.SimpleJsonResults, run *sarif.Run, simplifiedOutput bool) error {
	for _, vulnerability := range jsonTable.Vulnerabilities {
		sarifProperties, err := getSarifProperties(vulnerability, simplifiedOutput)
		if err != nil {
			return err
		}
		err = addScanResultsToSarifRun(run, sarifProperties.Severity, vulnerability.IssueId, sarifProperties.Headline, sarifProperties.Description, vulnerability.Technology)
		if err != nil {
			return err
		}
	}

	return nil
}

func getDirectDependenciesFormatted(directDependencies []formats.ComponentRow) string {
	var formattedDirectDependencies strings.Builder
	for _, dependency := range directDependencies {
		formattedDirectDependencies.WriteString(fmt.Sprintf("`%s:%s`, ", dependency.Name, dependency.Version))
	}
	return strings.TrimSuffix(formattedDirectDependencies.String(), ", ")
}

func getDescription(formattedDirectDependencies, maxCveScore string, fixedVersions []string) string {
	descriptionFixVersions := "No fix available"
	if len(fixedVersions) > 0 {
		descriptionFixVersions = strings.Join(fixedVersions, ", ")
	}
	return fmt.Sprintf("| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---        |    :----:   |          ---: |\n| %s      | %s       | %s   |",
		maxCveScore, formattedDirectDependencies, descriptionFixVersions)
}

func getMinimalFixVersion(fixVersions []string) string {
	if len(fixVersions) > 0 {
		return fixVersions[0]
	}
	return ""
}

// Adding the Xray scan results details to the sarif struct, for each issue found in the scan
func addScanResultsToSarifRun(run *sarif.Run, severity, issueId, impactedPackage, description string, technology coreutils.Technology) error {
	techPackageDescriptor := technology.GetPackageDescriptor()
	pb := sarif.NewPropertyBag()
	if severity != missingCveScore {
		pb.Add("security-severity", severity)
	}
	run.AddRule(issueId).
		WithProperties(pb.Properties).
		WithMarkdownHelp(description)
	run.CreateResultForRule(issueId).
		WithMessage(sarif.NewTextMessage(impactedPackage)).
		AddLocation(
			sarif.NewLocationWithPhysicalLocation(
				sarif.NewPhysicalLocation().
					WithArtifactLocation(
						sarif.NewSimpleArtifactLocation(techPackageDescriptor),
					),
			),
		)

	return nil
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
