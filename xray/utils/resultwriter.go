package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jfrog/gofrog/version"
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
	Type                string
	CodeFlows           [][]formats.SourceCodeLocationRow
}

type ResultsWriter struct {
	// The scan results.
	results *ExtendedScanResults
	// simpleJsonError  Errors to be added to output of the SimpleJson format.
	simpleJsonError []formats.SimpleJsonError
	// format  The output format.
	format OutputFormat
	// includeVulnerabilities  If true, include all vulnerabilities as part of the output. Else, include violations only.
	includeVulnerabilities bool
	// includeLicenses  If true, also include license violations as part of the output.
	includeLicenses bool
	// isMultipleRoots  multipleRoots is set to true, in case the given results array contains (or may contain) results of several projects (like in binary scan).
	isMultipleRoots bool
	// printExtended, If true, show extended results.
	printExtended bool
	// The scanType (binary,docker,dependency)
	scanType services.ScanType
	// messages - Option array of messages, to be displayed if the format is Table
	messages []string
	// Maps layer hash to command,used for docker scan.
	dockerCommandsMapping map[string]services.DockerCommandDetails
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

func (rw *ResultsWriter) SetDockerCommandsMapping(mapping map[string]services.DockerCommandDetails) *ResultsWriter {
	rw.dockerCommandsMapping = mapping
	return rw
}

// PrintScanResults prints the scan results in the specified format.
// Note that errors are printed only with SimpleJson format.
func (rw *ResultsWriter) PrintScanResults() error {
	switch rw.format {
	case Table:
		return rw.printScanResultsTables()
	case SimpleJson:
		jsonTable, err := rw.convertScanToSimpleJson(false)
		if err != nil {
			return err
		}
		return PrintJson(jsonTable)
	case Json:
		return PrintJson(rw.results.getXrayScanResults())
	case Sarif:
		sarifFile, err := rw.generateSarifFileFromScan(false, "JFrog Security", coreutils.JFrogComUrl+"xray/")
		if err != nil {
			return err
		}
		log.Output(sarifFile)
	}
	return nil
}

func (rw *ResultsWriter) generateSarifFileFromScan(markdownOutput bool, scanningTool, toolURI string) (string, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	run := sarif.NewRunWithInformationURI(scanningTool, toolURI)
	if err = rw.convertScanToSarif(run, markdownOutput); err != nil {
		return "", err
	}
	report.AddRun(run)
	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return clientUtils.IndentJson(out), nil
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
		err = PrintVulnerabilitiesTable(vulnerabilities, rw.results, rw.isMultipleRoots, rw.printExtended, rw.scanType, rw.dockerCommandsMapping)
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
	if !version.NewVersion(AnalyzerManagerVersion).AtLeast(MinAnalyzerManagerVersionForSast) {
		return
	}
	return PrintSastTable(rw.results.SastResults, rw.results.EntitledForJas)
}

func (rw *ResultsWriter) convertScanToSimpleJson(simplifiedOutput bool) (formats.SimpleJsonResults, error) {
	extendedResults := rw.results
	violations, vulnerabilities, licenses := SplitScanResults(extendedResults.XrayResults)
	jsonTable := formats.SimpleJsonResults{}
	if len(vulnerabilities) > 0 {
		vulJsonTable, err := PrepareVulnerabilities(vulnerabilities, extendedResults, rw.isMultipleRoots, simplifiedOutput)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Vulnerabilities = vulJsonTable
	}
	if len(violations) > 0 {
		secViolationsJsonTable, licViolationsJsonTable, opRiskViolationsJsonTable, err := PrepareViolations(violations, extendedResults, rw.isMultipleRoots, simplifiedOutput)
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
	if len(extendedResults.SastResults) > 0 {
		sastRows := PrepareSast(extendedResults.SastResults)
		jsonTable.Sast = sastRows
	}
	if rw.includeLicenses {
		licJsonTable, err := PrepareLicenses(licenses)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Licenses = licJsonTable
	}
	jsonTable.Errors = rw.simpleJsonError

	return jsonTable, nil
}

func (rw *ResultsWriter) convertScanToSarif(run *sarif.Run, markdownOutput bool) error {
	jsonTable, err := rw.convertScanToSimpleJson(markdownOutput)
	if err != nil {
		return err
	}
	if len(jsonTable.Vulnerabilities) > 0 || len(jsonTable.SecurityViolations) > 0 {
		if err = convertToVulnerabilityOrViolationSarif(run, &jsonTable, markdownOutput); err != nil {
			return err
		}
	}
	return convertToSourceCodeResultSarif(run, &jsonTable, markdownOutput)
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
func convertToVulnerabilityOrViolationSarif(run *sarif.Run, jsonTable *formats.SimpleJsonResults, markdownOutput bool) error {
	if len(jsonTable.SecurityViolations) > 0 {
		return convertViolationsToSarif(jsonTable, run, markdownOutput)
	}
	return convertVulnerabilitiesToSarif(jsonTable, run, markdownOutput)
}

func convertToSourceCodeResultSarif(run *sarif.Run, jsonTable *formats.SimpleJsonResults, markdownOutput bool) (err error) {
	for _, secret := range jsonTable.Secrets {
		properties := getSourceCodeProperties(secret, markdownOutput, Secrets)
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return
		}
	}

	for _, iac := range jsonTable.Iacs {
		properties := getSourceCodeProperties(iac, markdownOutput, IaC)
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return
		}
	}

	for _, sast := range jsonTable.Sast {
		properties := getSourceCodeProperties(sast, markdownOutput, Sast)
		if err = addPropertiesToSarifRun(run, &properties); err != nil {
			return
		}
	}
	return
}

func getSourceCodeProperties(sourceCodeIssue formats.SourceCodeRow, markdownOutput bool, scanType JasScanType) sarifProperties {
	file := strings.TrimPrefix(sourceCodeIssue.File, string(os.PathSeparator))
	mapSeverityToScore := map[string]string{
		"":         "0.0",
		"unknown":  "0.0",
		"low":      "3.9",
		"medium":   "6.9",
		"high":     "8.9",
		"critical": "10",
	}
	severity := mapSeverityToScore[strings.ToLower(sourceCodeIssue.Severity)]

	headline := ""
	secretOrFinding := ""
	switch scanType {
	case IaC:
		headline = "Infrastructure as Code Vulnerability"
		secretOrFinding = "Finding"
	case Sast:
		headline = sourceCodeIssue.Text
		secretOrFinding = "Finding"
	case Secrets:
		headline = "Potential Secret Exposed"
		secretOrFinding = "Secret"
	}

	markdownDescription := ""
	if markdownOutput {
		headerRow := fmt.Sprintf("| Severity | File | Line:Column | %s |\n", secretOrFinding)
		separatorRow := "| :---: | :---: | :---: | :---: |\n"
		tableHeader := headerRow + separatorRow
		markdownDescription = tableHeader + fmt.Sprintf("| %s | %s | %s | %s |", sourceCodeIssue.Severity, file, sourceCodeIssue.LineColumn, sourceCodeIssue.Text)
	}
	return sarifProperties{
		Headline:            headline,
		Severity:            severity,
		Description:         sourceCodeIssue.Text,
		MarkdownDescription: markdownDescription,
		File:                file,
		LineColumn:          sourceCodeIssue.LineColumn,
		Type:                sourceCodeIssue.Type,
		CodeFlows:           sourceCodeIssue.CodeFlow,
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
	if applicable == string(NotScanned) {
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
	location, err := getSarifLocation(properties.File, properties.LineColumn)
	if err != nil {
		return err
	}
	codeFlows, err := getCodeFlowProperties(properties)
	if err != nil {
		return err
	}
	ruleID := generateSarifRuleID(properties)
	run.AddRule(ruleID).
		WithDescription(description).
		WithProperties(pb.Properties).
		WithMarkdownHelp(markdownDescription)
	run.CreateResultForRule(ruleID).
		WithCodeFlows(codeFlows).
		WithMessage(sarif.NewTextMessage(properties.Headline)).
		AddLocation(location)
	return nil
}

func getSarifLocation(file, lineCol string) (location *sarif.Location, err error) {
	line := 0
	column := 0
	if lineCol != "" {
		lineColumn := strings.Split(lineCol, ":")
		if line, err = strconv.Atoi(lineColumn[0]); err != nil {
			return
		}
		if column, err = strconv.Atoi(lineColumn[1]); err != nil {
			return
		}
	}
	location = sarif.NewLocationWithPhysicalLocation(
		sarif.NewPhysicalLocation().
			WithArtifactLocation(
				sarif.NewSimpleArtifactLocation(file),
			).WithRegion(
			sarif.NewSimpleRegion(line, line).
				WithStartColumn(column)),
	)
	return
}

func getCodeFlowProperties(properties *sarifProperties) (flows []*sarif.CodeFlow, err error) {
	for _, codeFlow := range properties.CodeFlows {
		if len(codeFlow) == 0 {
			continue
		}
		converted := sarif.NewCodeFlow()
		locations := []*sarif.ThreadFlowLocation{}
		for _, location := range codeFlow {
			var convertedLocation *sarif.Location
			if convertedLocation, err = getSarifLocation(location.File, location.LineColumn); err != nil {
				return
			}
			locations = append(locations, sarif.NewThreadFlowLocation().WithLocation(convertedLocation))
		}

		converted.AddThreadFlow(sarif.NewThreadFlow().WithLocations(locations))
		flows = append(flows, converted)
	}
	return
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
