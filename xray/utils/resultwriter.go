package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
	Applicable          string
	Cves                string
	Headline            string
	Severity            string
	Description         string
	MarkdownDescription string
	XrayID              string
	File                string
	LineColumn          string
	SecretsOrIacType    string
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
		jsonTable, err := convertScanToSimpleJson(results, simpleJsonError, isMultipleRoots, includeLicenses, false)
		if err != nil {
			return err
		}
		return PrintJson(jsonTable)
	case Json:
		return PrintJson(results.getXrayScanResults())
	case Sarif:
		sarifFile, err := GenerateSarifFileFromScan(results, isMultipleRoots, false, "JFrog Security", coreutils.JFrogComUrl+"xray/")
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
		if err = PrintLicensesTable(licenses, printExtended, scan); err != nil {
			return
		}
	}
	if err = PrintSecretsTable(results.SecretsScanResults, results.EligibleForSecretScan); err != nil {
		return
	}
	return PrintIacTable(results.IacScanResults, results.EligibleForIacScan)
}

func printMessages(messages []string) {
	for _, m := range messages {
		printMessage(m)
	}
}

func printMessage(message string) {
	log.Output("💬", message)
}

func GenerateSarifFileFromScan(extendedResults *ExtendedScanResults, isMultipleRoots, markdownOutput bool, scanningTool, toolURI string) (string, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	run := sarif.NewRunWithInformationURI(scanningTool, toolURI)
	if err = convertScanToSarif(run, extendedResults, isMultipleRoots, markdownOutput); err != nil {
		return "", err
	}
	report.AddRun(run)
	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return clientUtils.IndentJson(out), nil
}

func convertScanToSimpleJson(extendedResults *ExtendedScanResults, errors []formats.SimpleJsonError, isMultipleRoots, includeLicenses, simplifiedOutput bool) (formats.SimpleJsonResults, error) {
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
	if len(extendedResults.SecretsScanResults) > 0 {
		secretsRows := PrepareSecrets(extendedResults.SecretsScanResults)
		jsonTable.Secrets = secretsRows
	}
	if len(extendedResults.IacScanResults) > 0 {
		iacRows := PrepareIacs(extendedResults.IacScanResults)
		jsonTable.Iacs = iacRows
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

func convertScanToSarif(run *sarif.Run, extendedResults *ExtendedScanResults, isMultipleRoots, markdownOutput bool) error {
	var errors []formats.SimpleJsonError
	jsonTable, err := convertScanToSimpleJson(extendedResults, errors, isMultipleRoots, true, markdownOutput)
	if err != nil {
		return err
	}
	if len(jsonTable.Vulnerabilities) > 0 || len(jsonTable.SecurityViolations) > 0 {
		if err = convertToVulnerabilityOrViolationSarif(run, &jsonTable, markdownOutput); err != nil {
			return err
		}
	}
	return convertToIacOrSecretsSarif(run, &jsonTable, markdownOutput)
}

func convertToVulnerabilityOrViolationSarif(run *sarif.Run, jsonTable *formats.SimpleJsonResults, markdownOutput bool) error {
	if len(jsonTable.SecurityViolations) > 0 {
		return convertViolationsToSarif(jsonTable, run, markdownOutput)
	}
	return convertVulnerabilitiesToSarif(jsonTable, run, markdownOutput)
}

func convertToIacOrSecretsSarif(run *sarif.Run, jsonTable *formats.SimpleJsonResults, markdownOutput bool) error {
	var err error
	for _, secret := range jsonTable.Secrets {
		properties := getIacOrSecretsProperties(secret, markdownOutput, true)
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return err
		}
	}

	for _, iac := range jsonTable.Iacs {
		properties := getIacOrSecretsProperties(iac, markdownOutput, false)
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return err
		}
	}
	return err
}

func getIacOrSecretsProperties(secretOrIac formats.IacSecretsRow, markdownOutput, isSecret bool) sarifProperties {
	file := strings.TrimPrefix(secretOrIac.File, string(os.PathSeparator))
	mapSeverityToScore := map[string]string{
		"":         "0.0",
		"low":      "3.9",
		"medium":   "6.9",
		"high":     "8.9",
		"critical": "10",
	}
	severity := mapSeverityToScore[strings.ToLower(secretOrIac.Severity)]
	markdownDescription := ""
	headline := "Infrastructure as Code Vulnerability"
	secretOrFinding := "Finding"
	if isSecret {
		secretOrFinding = "Secret"
		headline = "Potential Secret Exposed"
	}
	if markdownOutput {
		headerRow := fmt.Sprintf("| Severity | File | Line:Column | %s |\n", secretOrFinding)
		separatorRow := "| :---: | :---: | :---: | :---: |\n"
		tableHeader := headerRow + separatorRow
		markdownDescription = tableHeader + fmt.Sprintf("| %s | %s | %s | %s |", secretOrIac.Severity, file, secretOrIac.LineColumn, secretOrIac.Text)
	}
	return sarifProperties{
		Headline:            headline,
		Severity:            severity,
		Description:         secretOrIac.Text,
		MarkdownDescription: markdownDescription,
		File:                file,
		LineColumn:          secretOrIac.LineColumn,
		SecretsOrIacType:    secretOrIac.Type,
	}
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

func getVulnerabilityOrViolationSarifHeadline(depName, version, key string) string {
	return fmt.Sprintf("[%s] %s %s", key, depName, version)
}

func convertViolationsToSarif(jsonTable *formats.SimpleJsonResults, run *sarif.Run, markdownOutput bool) error {
	for _, violation := range jsonTable.SecurityViolations {
		properties, err := getViolatedDepsSarifProps(violation, markdownOutput)
		if err != nil {
			return err
		}
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return err
		}
	}
	for _, license := range jsonTable.LicensesViolations {
		if err := addPropertiesToSarifRun(run,
			&sarifProperties{
				Severity: license.Severity,
				Headline: getVulnerabilityOrViolationSarifHeadline(license.LicenseKey, license.ImpactedDependencyName, license.ImpactedDependencyVersion)}); err != nil {
			return err
		}
	}

	return nil
}

func getViolatedDepsSarifProps(vulnerabilityRow formats.VulnerabilityOrViolationRow, markdownOutput bool) (sarifProperties, error) {
	cves := getCves(vulnerabilityRow.Cves, vulnerabilityRow.IssueId)
	headline := getVulnerabilityOrViolationSarifHeadline(vulnerabilityRow.ImpactedDependencyName, vulnerabilityRow.ImpactedDependencyVersion, cves)
	maxCveScore, err := findMaxCVEScore(vulnerabilityRow.Cves)
	if err != nil {
		return sarifProperties{}, err
	}
	formattedDirectDependencies, err := getDirectDependenciesFormatted(vulnerabilityRow.Components)
	if err != nil {
		return sarifProperties{}, err
	}
	markdownDescription := ""
	if markdownOutput {
		markdownDescription = getSarifTableDescription(formattedDirectDependencies, maxCveScore, vulnerabilityRow.Applicable, vulnerabilityRow.FixedVersions) + "\n"
	}
	return sarifProperties{
		Applicable:          vulnerabilityRow.Applicable,
		Cves:                cves,
		Headline:            headline,
		Severity:            maxCveScore,
		Description:         vulnerabilityRow.Summary,
		MarkdownDescription: markdownDescription,
		File:                vulnerabilityRow.Technology.GetPackageDescriptor(),
	}, err
}

func convertVulnerabilitiesToSarif(jsonTable *formats.SimpleJsonResults, run *sarif.Run, simplifiedOutput bool) error {
	for _, vulnerability := range jsonTable.Vulnerabilities {
		properties, err := getViolatedDepsSarifProps(vulnerability, simplifiedOutput)
		if err != nil {
			return err
		}
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return err
		}
	}

	return nil
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
	if applicable == "" {
		return fmt.Sprintf("| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| %s      | %s       | %s   |",
			maxCveScore, formattedDirectDependencies, descriptionFixVersions)
	}
	return fmt.Sprintf("| Severity Score | Contextual Analysis | Direct Dependencies | Fixed Versions     |\n|  :---:  |  :---:  |  :---:  |  :---:  |\n| %s      | %s       | %s       | %s   |",
		maxCveScore, applicable, formattedDirectDependencies, descriptionFixVersions)
}

// Adding the Xray scan results details to the sarif struct, for each issue found in the scan
func addPropertiesToSarifRun(run *sarif.Run, properties *sarifProperties) error {
	pb := sarif.NewPropertyBag()
	if properties.Severity != missingCveScore {
		pb.Add("security-severity", properties.Severity)
	}
	description := properties.Description
	markdownDescription := properties.MarkdownDescription
	if markdownDescription != "" {
		description = ""
	}
	line := 0
	column := 0
	var err error
	if properties.LineColumn != "" {
		lineColumn := strings.Split(properties.LineColumn, ":")
		if line, err = strconv.Atoi(lineColumn[0]); err != nil {
			return err
		}
		if column, err = strconv.Atoi(lineColumn[1]); err != nil {
			return err
		}
	}
	ruleID := generateSarifRuleID(properties)
	run.AddRule(ruleID).
		WithDescription(description).
		WithProperties(pb.Properties).
		WithMarkdownHelp(markdownDescription)
	run.CreateResultForRule(ruleID).
		WithMessage(sarif.NewTextMessage(properties.Headline)).
		AddLocation(
			sarif.NewLocationWithPhysicalLocation(
				sarif.NewPhysicalLocation().
					WithArtifactLocation(
						sarif.NewSimpleArtifactLocation(properties.File),
					).WithRegion(
					sarif.NewSimpleRegion(line, line).
						WithStartColumn(column)),
			),
		)
	return nil
}

func generateSarifRuleID(properties *sarifProperties) string {
	switch {
	case properties.Cves != "":
		return properties.Cves
	case properties.XrayID != "":
		return properties.XrayID
	default:
		return properties.File
	}
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
