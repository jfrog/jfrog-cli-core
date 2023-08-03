package utils

import (
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
	"golang.org/x/exp/maps"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"

	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	rootIndex                  = 0
	directDependencyIndex      = 1
	directDependencyPathLength = 2
)

// PrintViolationsTable prints the violations in 4 tables: security violations, license compliance violations, operational risk violations and ignore rule URLs.
// Set multipleRoots to true in case the given violations array contains (or may contain) results of several projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
// Set printExtended to true to print fields with 'extended' tag.
// If the scan argument is set to true, print the scan tables.
func PrintViolationsTable(violations []services.Violation, extendedResults *ExtendedScanResults, multipleRoots, printExtended, scan bool) error {
	securityViolationsRows, licenseViolationsRows, operationalRiskViolationsRows, err := prepareViolations(violations, extendedResults, multipleRoots, true, true)
	if err != nil {
		return err
	}
	// Print tables, if scan is true; print the scan tables.
	if scan {
		err = coreutils.PrintTable(formats.ConvertToVulnerabilityScanTableRow(securityViolationsRows), "Security Violations", "No security violations were found", printExtended)
		if err != nil {
			return err
		}
		err = coreutils.PrintTable(formats.ConvertToLicenseViolationScanTableRow(licenseViolationsRows), "License Compliance Violations", "No license compliance violations were found", printExtended)
		if err != nil {
			return err
		}
		if len(operationalRiskViolationsRows) > 0 {
			return coreutils.PrintTable(formats.ConvertToOperationalRiskViolationScanTableRow(operationalRiskViolationsRows), "Operational Risk Violations", "No operational risk violations were found", printExtended)
		}
	} else {
		err = coreutils.PrintTable(formats.ConvertToVulnerabilityTableRow(securityViolationsRows), "Security Violations", "No security violations were found", printExtended)
		if err != nil {
			return err
		}
		err = coreutils.PrintTable(formats.ConvertToLicenseViolationTableRow(licenseViolationsRows), "License Compliance Violations", "No license compliance violations were found", printExtended)
		if err != nil {
			return err
		}
		if len(operationalRiskViolationsRows) > 0 {
			return coreutils.PrintTable(formats.ConvertToOperationalRiskViolationTableRow(operationalRiskViolationsRows), "Operational Risk Violations", "No operational risk violations were found", printExtended)
		}
	}
	return nil
}

// Prepare violations for all non-table formats (without style or emoji)
func PrepareViolations(violations []services.Violation, extendedResults *ExtendedScanResults, multipleRoots, simplifiedOutput bool) ([]formats.VulnerabilityOrViolationRow, []formats.LicenseViolationRow, []formats.OperationalRiskViolationRow, error) {
	return prepareViolations(violations, extendedResults, multipleRoots, false, simplifiedOutput)
}

func prepareViolations(violations []services.Violation, extendedResults *ExtendedScanResults, multipleRoots, isTable, simplifiedOutput bool) ([]formats.VulnerabilityOrViolationRow, []formats.LicenseViolationRow, []formats.OperationalRiskViolationRow, error) {
	if simplifiedOutput {
		violations = simplifyViolations(violations, multipleRoots)
	}
	var securityViolationsRows []formats.VulnerabilityOrViolationRow
	var licenseViolationsRows []formats.LicenseViolationRow
	var operationalRiskViolationsRows []formats.OperationalRiskViolationRow
	for _, violation := range violations {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, impactPaths, err := splitComponents(violation.Components)
		if err != nil {
			return nil, nil, nil, err
		}
		switch violation.ViolationType {
		case "security":
			cves := convertCves(violation.Cves)
			applicableValue := getApplicableCveValue(extendedResults, cves)
			currSeverity := GetSeverity(violation.Severity, applicableValue)
			jfrogResearchInfo := convertJfrogResearchInformation(violation.ExtendedInformation)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				securityViolationsRows = append(securityViolationsRows,
					formats.VulnerabilityOrViolationRow{
						Summary:                   violation.Summary,
						Severity:                  currSeverity.printableTitle(isTable),
						SeverityNumValue:          currSeverity.numValue,
						ImpactedDependencyName:    impactedPackagesNames[compIndex],
						ImpactedDependencyVersion: impactedPackagesVersions[compIndex],
						ImpactedDependencyType:    impactedPackagesTypes[compIndex],
						FixedVersions:             fixedVersions[compIndex],
						Components:                components[compIndex],
						Cves:                      cves,
						IssueId:                   violation.IssueId,
						References:                violation.References,
						JfrogResearchInformation:  jfrogResearchInfo,
						ImpactPaths:               impactPaths[compIndex],
						Technology:                coreutils.Technology(violation.Technology),
						Applicable:                printApplicableCveValue(applicableValue, isTable),
					},
				)
			}
		case "license":
			currSeverity := GetSeverity(violation.Severity, ApplicabilityUndeterminedStringValue)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				licenseViolationsRows = append(licenseViolationsRows,
					formats.LicenseViolationRow{
						LicenseKey:                violation.LicenseKey,
						Severity:                  currSeverity.printableTitle(isTable),
						SeverityNumValue:          currSeverity.numValue,
						ImpactedDependencyName:    impactedPackagesNames[compIndex],
						ImpactedDependencyVersion: impactedPackagesVersions[compIndex],
						ImpactedDependencyType:    impactedPackagesTypes[compIndex],
						Components:                components[compIndex],
					},
				)
			}
		case "operational_risk":
			currSeverity := GetSeverity(violation.Severity, ApplicabilityUndeterminedStringValue)
			violationOpRiskData := getOperationalRiskViolationReadableData(violation)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				operationalRiskViolationsRow := &formats.OperationalRiskViolationRow{
					Severity:                  currSeverity.printableTitle(isTable),
					SeverityNumValue:          currSeverity.numValue,
					ImpactedDependencyName:    impactedPackagesNames[compIndex],
					ImpactedDependencyVersion: impactedPackagesVersions[compIndex],
					ImpactedDependencyType:    impactedPackagesTypes[compIndex],
					Components:                components[compIndex],
					IsEol:                     violationOpRiskData.isEol,
					Cadence:                   violationOpRiskData.cadence,
					Commits:                   violationOpRiskData.commits,
					Committers:                violationOpRiskData.committers,
					NewerVersions:             violationOpRiskData.newerVersions,
					LatestVersion:             violationOpRiskData.latestVersion,
					RiskReason:                violationOpRiskData.riskReason,
					EolMessage:                violationOpRiskData.eolMessage,
				}
				operationalRiskViolationsRows = append(operationalRiskViolationsRows, *operationalRiskViolationsRow)
			}
		default:
			// Unsupported type, ignore
		}
	}

	// Sort the rows by severity and whether the row contains fixed versions
	sortVulnerabilityOrViolationRows(securityViolationsRows)
	sort.Slice(licenseViolationsRows, func(i, j int) bool {
		return licenseViolationsRows[i].SeverityNumValue > licenseViolationsRows[j].SeverityNumValue
	})
	sort.Slice(operationalRiskViolationsRows, func(i, j int) bool {
		return operationalRiskViolationsRows[i].SeverityNumValue > operationalRiskViolationsRows[j].SeverityNumValue
	})

	return securityViolationsRows, licenseViolationsRows, operationalRiskViolationsRows, nil
}

// PrintVulnerabilitiesTable prints the vulnerabilities in a table.
// Set multipleRoots to true in case the given vulnerabilities array contains (or may contain) results of several projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// Set printExtended to true to print fields with 'extended' tag.
// If the scan argument is set to true, print the scan tables.
func PrintVulnerabilitiesTable(vulnerabilities []services.Vulnerability, extendedResults *ExtendedScanResults, multipleRoots, printExtended, scan bool) error {
	vulnerabilitiesRows, err := prepareVulnerabilities(vulnerabilities, extendedResults, multipleRoots, true, true)
	if err != nil {
		return err
	}

	if scan {
		return coreutils.PrintTable(formats.ConvertToVulnerabilityScanTableRow(vulnerabilitiesRows), "Vulnerabilities", "✨ No vulnerabilities were found ✨", printExtended)
	}

	return coreutils.PrintTable(formats.ConvertToVulnerabilityTableRow(vulnerabilitiesRows), "Vulnerabilities", "✨ No vulnerabilities were found ✨", printExtended)
}

// Prepare vulnerabilities for all non-table formats (without style or emoji)
func PrepareVulnerabilities(vulnerabilities []services.Vulnerability, extendedResults *ExtendedScanResults, multipleRoots, simplifiedOutput bool) ([]formats.VulnerabilityOrViolationRow, error) {
	return prepareVulnerabilities(vulnerabilities, extendedResults, multipleRoots, false, simplifiedOutput)
}

func prepareVulnerabilities(vulnerabilities []services.Vulnerability, extendedResults *ExtendedScanResults, multipleRoots, isTable, simplifiedOutput bool) ([]formats.VulnerabilityOrViolationRow, error) {
	if simplifiedOutput {
		vulnerabilities = simplifyVulnerabilities(vulnerabilities, multipleRoots)
	}
	var vulnerabilitiesRows []formats.VulnerabilityOrViolationRow
	for _, vulnerability := range vulnerabilities {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, impactPaths, err := splitComponents(vulnerability.Components)
		if err != nil {
			return nil, err
		}
		cves := convertCves(vulnerability.Cves)
		applicableValue := getApplicableCveValue(extendedResults, cves)
		currSeverity := GetSeverity(vulnerability.Severity, applicableValue)
		jfrogResearchInfo := convertJfrogResearchInformation(vulnerability.ExtendedInformation)
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			vulnerabilitiesRows = append(vulnerabilitiesRows,
				formats.VulnerabilityOrViolationRow{
					Summary:                   vulnerability.Summary,
					Severity:                  currSeverity.printableTitle(isTable),
					SeverityNumValue:          currSeverity.numValue,
					ImpactedDependencyName:    impactedPackagesNames[compIndex],
					ImpactedDependencyVersion: impactedPackagesVersions[compIndex],
					ImpactedDependencyType:    impactedPackagesTypes[compIndex],
					FixedVersions:             fixedVersions[compIndex],
					Components:                components[compIndex],
					Cves:                      cves,
					IssueId:                   vulnerability.IssueId,
					References:                vulnerability.References,
					JfrogResearchInformation:  jfrogResearchInfo,
					ImpactPaths:               impactPaths[compIndex],
					Technology:                coreutils.Technology(vulnerability.Technology),
					Applicable:                printApplicableCveValue(applicableValue, isTable),
				},
			)
		}
	}

	sortVulnerabilityOrViolationRows(vulnerabilitiesRows)
	return vulnerabilitiesRows, nil
}

func sortVulnerabilityOrViolationRows(rows []formats.VulnerabilityOrViolationRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SeverityNumValue != rows[j].SeverityNumValue {
			return rows[i].SeverityNumValue > rows[j].SeverityNumValue
		}
		return len(rows[i].FixedVersions) > 0 && len(rows[j].FixedVersions) > 0
	})
}

// PrintLicensesTable prints the licenses in a table.
// Set multipleRoots to true in case the given licenses array contains (or may contain) results of several projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// Set printExtended to true to print fields with 'extended' tag.
// If the scan argument is set to true, print the scan tables.
func PrintLicensesTable(licenses []services.License, printExtended, scan bool) error {
	licensesRows, err := PrepareLicenses(licenses)
	if err != nil {
		return err
	}
	if scan {
		return coreutils.PrintTable(formats.ConvertToLicenseScanTableRow(licensesRows), "Licenses", "No licenses were found", printExtended)
	}
	return coreutils.PrintTable(formats.ConvertToLicenseTableRow(licensesRows), "Licenses", "No licenses were found", printExtended)
}

func PrepareLicenses(licenses []services.License) ([]formats.LicenseRow, error) {
	var licensesRows []formats.LicenseRow
	for _, license := range licenses {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, _, components, impactPaths, err := splitComponents(license.Components)
		if err != nil {
			return nil, err
		}
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			licensesRows = append(licensesRows,
				formats.LicenseRow{
					LicenseKey:                license.Key,
					ImpactedDependencyName:    impactedPackagesNames[compIndex],
					ImpactedDependencyVersion: impactedPackagesVersions[compIndex],
					ImpactedDependencyType:    impactedPackagesTypes[compIndex],
					Components:                components[compIndex],
					ImpactPaths:               impactPaths[compIndex],
				},
			)
		}
	}

	return licensesRows, nil
}

// Prepare secrets for all non-table formats (without style or emoji)
func PrepareSecrets(secrets []IacOrSecretResult) []formats.IacSecretsRow {
	return prepareSecrets(secrets, false)
}

func prepareSecrets(secrets []IacOrSecretResult, isTable bool) []formats.IacSecretsRow {
	var secretsRows []formats.IacSecretsRow
	for _, secret := range secrets {
		currSeverity := GetSeverity(secret.Severity, ApplicableStringValue)
		secretsRows = append(secretsRows,
			formats.IacSecretsRow{
				Severity:         currSeverity.printableTitle(isTable),
				SeverityNumValue: currSeverity.numValue,
				File:             secret.File,
				LineColumn:       secret.LineColumn,
				Text:             secret.Text,
				Type:             secret.Type,
			},
		)
	}

	sort.Slice(secretsRows, func(i, j int) bool {
		return secretsRows[i].SeverityNumValue > secretsRows[j].SeverityNumValue
	})

	return secretsRows
}

func PrintSecretsTable(secrets []IacOrSecretResult, entitledForSecretsScan bool) error {
	if entitledForSecretsScan {
		secretsRows := prepareSecrets(secrets, true)
		return coreutils.PrintTable(formats.ConvertToSecretsTableRow(secretsRows), "Secrets",
			"✨ No secrets were found ✨", false)
	}
	return nil
}

// Prepare iacs for all non-table formats (without style or emoji)
func PrepareIacs(iacs []IacOrSecretResult) []formats.IacSecretsRow {
	return prepareIacs(iacs, false)
}

func prepareIacs(iacs []IacOrSecretResult, isTable bool) []formats.IacSecretsRow {
	var iacRows []formats.IacSecretsRow
	for _, iac := range iacs {
		currSeverity := GetSeverity(iac.Severity, ApplicableStringValue)
		iacRows = append(iacRows,
			formats.IacSecretsRow{
				Severity:         currSeverity.printableTitle(isTable),
				SeverityNumValue: currSeverity.numValue,
				File:             iac.File,
				LineColumn:       iac.LineColumn,
				Text:             iac.Text,
				Type:             iac.Type,
			},
		)
	}

	sort.Slice(iacRows, func(i, j int) bool {
		return iacRows[i].SeverityNumValue > iacRows[j].SeverityNumValue
	})

	return iacRows
}

func PrintIacTable(iacs []IacOrSecretResult, entitledForIacScan bool) error {
	if entitledForIacScan {
		iacRows := prepareIacs(iacs, true)
		return coreutils.PrintTable(formats.ConvertToIacTableRow(iacRows), "Infrastructure as Code Vulnerabilities",
			"✨ No Infrastructure as Code vulnerabilities were found ✨", false)
	}
	return nil
}

func convertCves(cves []services.Cve) []formats.CveRow {
	var cveRows []formats.CveRow
	for _, cveObj := range cves {
		cveRows = append(cveRows, formats.CveRow{Id: cveObj.Id, CvssV2: cveObj.CvssV2Score, CvssV3: cveObj.CvssV3Score})
	}
	return cveRows
}

func convertJfrogResearchInformation(extendedInfo *services.ExtendedInformation) *formats.JfrogResearchInformation {
	if extendedInfo == nil {
		return nil
	}
	var severityReasons []formats.JfrogResearchSeverityReason
	for _, severityReason := range extendedInfo.JfrogResearchSeverityReasons {
		severityReasons = append(severityReasons, formats.JfrogResearchSeverityReason{
			Name:        severityReason.Name,
			Description: severityReason.Description,
			IsPositive:  severityReason.IsPositive,
		})
	}
	return &formats.JfrogResearchInformation{
		Summary:         extendedInfo.ShortDescription,
		Details:         extendedInfo.FullDescription,
		Severity:        extendedInfo.JfrogResearchSeverity,
		SeverityReasons: severityReasons,
		Remediation:     extendedInfo.Remediation,
	}
}

func splitComponents(impactedPackages map[string]services.Component) (impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes []string, fixedVersions [][]string, directComponents [][]formats.ComponentRow, impactPaths [][][]formats.ComponentRow, err error) {
	if len(impactedPackages) == 0 {
		err = errorutils.CheckErrorf("failed while parsing the response from Xray: violation doesn't have any components")
		return
	}
	for currCompId, currComp := range impactedPackages {
		currCompName, currCompVersion, currCompType := SplitComponentId(currCompId)
		impactedPackagesNames = append(impactedPackagesNames, currCompName)
		impactedPackagesVersions = append(impactedPackagesVersions, currCompVersion)
		impactedPackagesTypes = append(impactedPackagesTypes, currCompType)
		fixedVersions = append(fixedVersions, currComp.FixedVersions)
		currDirectComponents, currImpactPaths := getDirectComponentsAndImpactPaths(currComp.ImpactPaths)
		directComponents = append(directComponents, currDirectComponents)
		impactPaths = append(impactPaths, currImpactPaths)
	}
	return
}

var packageTypes = map[string]string{
	"gav":      "Maven",
	"docker":   "Docker",
	"rpm":      "RPM",
	"deb":      "Debian",
	"nuget":    "NuGet",
	"generic":  "Generic",
	"npm":      "npm",
	"pip":      "Python",
	"pypi":     "Python",
	"composer": "Composer",
	"go":       "Go",
	"alpine":   "Alpine",
}

// SplitComponentId splits a Xray component ID to the component name, version and package type.
// In case componentId doesn't contain a version, the returned version will be an empty string.
// In case componentId's format is invalid, it will be returned as the component name
// and empty strings will be returned instead of the version and the package type.
// Examples:
//  1. componentId: "gav://antparent:ant:1.6.5"
//     Returned values:
//     Component name: "antparent:ant"
//     Component version: "1.6.5"
//     Package type: "Maven"
//  2. componentId: "generic://sha256:244fd47e07d1004f0aed9c156aa09083c82bf8944eceb67c946ff7430510a77b/foo.jar"
//     Returned values:
//     Component name: "foo.jar"
//     Component version: ""
//     Package type: "Generic"
//  3. componentId: "invalid-comp-id"
//     Returned values:
//     Component name: "invalid-comp-id"
//     Component version: ""
//     Package type: ""
func SplitComponentId(componentId string) (string, string, string) {
	compIdParts := strings.Split(componentId, "://")
	// Invalid component ID
	if len(compIdParts) != 2 {
		return componentId, "", ""
	}

	packageType := compIdParts[0]
	packageId := compIdParts[1]

	// Generic identifier structure: generic://sha256:<Checksum>/name
	if packageType == "generic" {
		lastSlashIndex := strings.LastIndex(packageId, "/")
		return packageId[lastSlashIndex+1:], "", packageTypes[packageType]
	}

	var compName, compVersion string
	switch packageType {
	case "rpm":
		// RPM identifier structure: rpm://os-version:package:epoch-version:version
		// os-version is optional.
		splitCompId := strings.Split(packageId, ":")
		if len(splitCompId) >= 3 {
			compName = splitCompId[len(splitCompId)-3]
			compVersion = fmt.Sprintf("%s:%s", splitCompId[len(splitCompId)-2], splitCompId[len(splitCompId)-1])
		}
	default:
		// All other identifiers look like this: package-type://package-name:version.
		// Sometimes there's a namespace or a group before the package name, separated by a '/' or a ':'.
		lastColonIndex := strings.LastIndex(packageId, ":")

		if lastColonIndex != -1 {
			compName = packageId[:lastColonIndex]
			compVersion = packageId[lastColonIndex+1:]
		}
	}

	// If there's an error while parsing the component ID
	if compName == "" {
		compName = packageId
	}

	return compName, compVersion, packageTypes[packageType]
}

// Gets a slice of the direct dependencies or packages of the scanned component, that depends on the vulnerable package, and converts the impact paths.
func getDirectComponentsAndImpactPaths(impactPaths [][]services.ImpactPathNode) (components []formats.ComponentRow, impactPathsRows [][]formats.ComponentRow) {
	componentsMap := make(map[string]formats.ComponentRow)

	// The first node in the impact path is the scanned component itself. The second one is the direct dependency.
	impactPathLevel := 1
	for _, impactPath := range impactPaths {
		impactPathIndex := impactPathLevel
		if len(impactPath) <= impactPathLevel {
			impactPathIndex = len(impactPath) - 1
		}
		componentId := impactPath[impactPathIndex].ComponentId
		if _, exist := componentsMap[componentId]; !exist {
			compName, compVersion, _ := SplitComponentId(componentId)
			componentsMap[componentId] = formats.ComponentRow{Name: compName, Version: compVersion}
		}

		// Convert the impact path
		var compImpactPathRows []formats.ComponentRow
		for _, pathNode := range impactPath {
			nodeCompName, nodeCompVersion, _ := SplitComponentId(pathNode.ComponentId)
			compImpactPathRows = append(compImpactPathRows, formats.ComponentRow{
				Name:    nodeCompName,
				Version: nodeCompVersion,
			})
		}
		impactPathsRows = append(impactPathsRows, compImpactPathRows)
	}

	for _, row := range componentsMap {
		components = append(components, row)
	}
	return
}

type Severity struct {
	title    string
	numValue int
	style    color.Style
	emoji    string
}

func (s *Severity) printableTitle(isTable bool) string {
	if isTable && (log.IsStdOutTerminal() && log.IsColorsSupported() || os.Getenv("GITLAB_CI") != "") {
		return s.style.Render(s.emoji + s.title)
	}
	return s.title
}

var Severities = map[string]map[string]*Severity{
	"Critical": {
		ApplicableStringValue:                {emoji: "💀", title: "Critical", numValue: 12, style: color.New(color.BgLightRed, color.LightWhite)},
		ApplicabilityUndeterminedStringValue: {emoji: "💀", title: "Critical", numValue: 11, style: color.New(color.BgLightRed, color.LightWhite)},
		NotApplicableStringValue:             {emoji: "👌", title: "Critical", numValue: 4},
	},
	"High": {
		ApplicableStringValue:                {emoji: "🔥", title: "High", numValue: 10, style: color.New(color.Red)},
		ApplicabilityUndeterminedStringValue: {emoji: "🔥", title: "High", numValue: 9, style: color.New(color.Red)},
		NotApplicableStringValue:             {emoji: "👌", title: "High", numValue: 3},
	},
	"Medium": {
		ApplicableStringValue:                {emoji: "🎃", title: "Medium", numValue: 8, style: color.New(color.Yellow)},
		ApplicabilityUndeterminedStringValue: {emoji: "🎃", title: "Medium", numValue: 7, style: color.New(color.Yellow)},
		NotApplicableStringValue:             {emoji: "👌", title: "Medium", numValue: 2},
	},
	"Low": {
		ApplicableStringValue:                {emoji: "👻", title: "Low", numValue: 6},
		ApplicabilityUndeterminedStringValue: {emoji: "👻", title: "Low", numValue: 5},
		NotApplicableStringValue:             {emoji: "👌", title: "Low", numValue: 1},
	},
}

func (s *Severity) NumValue() int {
	return s.numValue
}

func (s *Severity) Emoji() string {
	return s.emoji
}

func GetSeveritiesFormat(severity string) (string, error) {
	formattedSeverity := cases.Title(language.Und).String(severity)
	if formattedSeverity != "" && Severities[formattedSeverity][ApplicableStringValue] == nil {
		return "", errorutils.CheckErrorf("only the following severities are supported: " + coreutils.ListToText(maps.Keys(Severities)))
	}

	return formattedSeverity, nil
}

func GetSeverity(severityTitle string, applicable string) *Severity {
	if Severities[severityTitle] == nil {
		return &Severity{title: severityTitle}
	}

	switch applicable {
	case NotApplicableStringValue:
		return Severities[severityTitle][NotApplicableStringValue]
	case ApplicableStringValue:
		return Severities[severityTitle][ApplicableStringValue]
	default:
		return Severities[severityTitle][ApplicabilityUndeterminedStringValue]
	}
}

type operationalRiskViolationReadableData struct {
	isEol         string
	cadence       string
	commits       string
	committers    string
	eolMessage    string
	riskReason    string
	latestVersion string
	newerVersions string
}

func getOperationalRiskViolationReadableData(violation services.Violation) *operationalRiskViolationReadableData {
	isEol, cadence, commits, committers, newerVersions, latestVersion := "N/A", "N/A", "N/A", "N/A", "N/A", "N/A"
	if violation.IsEol != nil {
		isEol = strconv.FormatBool(*violation.IsEol)
	}
	if violation.Cadence != nil {
		cadence = strconv.FormatFloat(*violation.Cadence, 'f', -1, 64)
	}
	if violation.Committers != nil {
		committers = strconv.FormatInt(int64(*violation.Committers), 10)
	}
	if violation.Commits != nil {
		commits = strconv.FormatInt(*violation.Commits, 10)
	}
	if violation.NewerVersions != nil {
		newerVersions = strconv.FormatInt(int64(*violation.NewerVersions), 10)
	}
	if violation.LatestVersion != "" {
		latestVersion = violation.LatestVersion
	}
	return &operationalRiskViolationReadableData{
		isEol:         isEol,
		cadence:       cadence,
		commits:       commits,
		committers:    committers,
		eolMessage:    violation.EolMessage,
		riskReason:    violation.RiskReason,
		latestVersion: latestVersion,
		newerVersions: newerVersions,
	}
}

// simplifyVulnerabilities returns a new slice of services.Vulnerability that contains only the unique vulnerabilities from the input slice
// The uniqueness of the vulnerabilities is determined by the GetUniqueKey function
func simplifyVulnerabilities(scanVulnerabilities []services.Vulnerability, multipleRoots bool) []services.Vulnerability {
	var uniqueVulnerabilities = make(map[string]*services.Vulnerability)
	for _, vulnerability := range scanVulnerabilities {
		for vulnerableComponentId := range vulnerability.Components {
			vulnerableDependency, vulnerableVersion, _ := SplitComponentId(vulnerableComponentId)
			packageKey := GetUniqueKey(vulnerableDependency, vulnerableVersion, vulnerability.IssueId, len(vulnerability.Components[vulnerableComponentId].FixedVersions) > 0)
			if uniqueVulnerability, exist := uniqueVulnerabilities[packageKey]; exist {
				fixedVersions := appendUniqueFixVersions(uniqueVulnerability.Components[vulnerableComponentId].FixedVersions, vulnerability.Components[vulnerableComponentId].FixedVersions...)
				impactPaths := appendUniqueImpactPaths(uniqueVulnerability.Components[vulnerableComponentId].ImpactPaths, vulnerability.Components[vulnerableComponentId].ImpactPaths, multipleRoots)
				uniqueVulnerabilities[packageKey].Components[vulnerableComponentId] = services.Component{
					FixedVersions: fixedVersions,
					ImpactPaths:   impactPaths,
				}
				continue
			}
			uniqueVulnerabilities[packageKey] = &services.Vulnerability{
				Cves:                vulnerability.Cves,
				Severity:            vulnerability.Severity,
				Components:          map[string]services.Component{vulnerableComponentId: vulnerability.Components[vulnerableComponentId]},
				IssueId:             vulnerability.IssueId,
				Technology:          vulnerability.Technology,
				ExtendedInformation: vulnerability.ExtendedInformation,
				Summary:             vulnerability.Summary,
			}
		}
	}
	// convert map to slice
	result := make([]services.Vulnerability, 0, len(uniqueVulnerabilities))
	for _, v := range uniqueVulnerabilities {
		result = append(result, *v)
	}
	return result
}

// simplifyViolations returns a new slice of services.Violations that contains only the unique violations from the input slice
// The uniqueness of the violations is determined by the GetUniqueKey function
func simplifyViolations(scanViolations []services.Violation, multipleRoots bool) []services.Violation {
	var uniqueViolations = make(map[string]*services.Violation)
	for _, violation := range scanViolations {
		for vulnerableComponentId := range violation.Components {
			vulnerableDependency, vulnerableVersion, _ := SplitComponentId(vulnerableComponentId)
			packageKey := GetUniqueKey(vulnerableDependency, vulnerableVersion, violation.IssueId, len(violation.Components[vulnerableComponentId].FixedVersions) > 0)
			if uniqueVulnerability, exist := uniqueViolations[packageKey]; exist {
				fixedVersions := appendUniqueFixVersions(uniqueVulnerability.Components[vulnerableComponentId].FixedVersions, violation.Components[vulnerableComponentId].FixedVersions...)
				impactPaths := appendUniqueImpactPaths(uniqueVulnerability.Components[vulnerableComponentId].ImpactPaths, violation.Components[vulnerableComponentId].ImpactPaths, multipleRoots)
				uniqueViolations[packageKey].Components[vulnerableComponentId] = services.Component{
					FixedVersions: fixedVersions,
					ImpactPaths:   impactPaths,
				}
				continue
			}
			uniqueViolations[packageKey] = &services.Violation{
				Severity:      violation.Severity,
				ViolationType: violation.ViolationType,
				Components:    map[string]services.Component{vulnerableComponentId: violation.Components[vulnerableComponentId]},
				WatchName:     violation.WatchName,
				IssueId:       violation.IssueId,
				Cves:          violation.Cves,
				LicenseKey:    violation.LicenseKey,
				LicenseName:   violation.LicenseName,
				Technology:    violation.Technology,
			}
		}
	}
	// convert map to slice
	result := make([]services.Violation, 0, len(uniqueViolations))
	for _, v := range uniqueViolations {
		result = append(result, *v)
	}
	return result
}

// appendImpactPathsWithoutDuplicates appends the elements of a source [][]ImpactPathNode struct to a target [][]ImpactPathNode, without adding any duplicate elements.
// This implementation uses the ComponentId field of the ImpactPathNode struct to check for duplicates, as it is guaranteed to be unique.
func appendUniqueImpactPaths(target [][]services.ImpactPathNode, source [][]services.ImpactPathNode, multipleRoots bool) [][]services.ImpactPathNode {
	if multipleRoots {
		return appendUniqueImpactPathsForMultipleRoots(target, source)
	}
	impactPathMap := make(map[string][]services.ImpactPathNode)
	for _, path := range target {
		// The first node component id is the key and the value is the whole path
		key := getImpactPathKey(path)
		impactPathMap[key] = path
	}

	for _, path := range source {
		key := getImpactPathKey(path)
		if _, exists := impactPathMap[key]; !exists {
			impactPathMap[key] = path
			target = append(target, path)
		}
	}
	return target
}

// getImpactPathKey return a key that is used as a key to identify and deduplicate impact paths.
// If an impact path length is equal to directDependencyPathLength, then the direct dependency is the key, and it's in the directDependencyIndex place.
func getImpactPathKey(path []services.ImpactPathNode) string {
	key := path[rootIndex].ComponentId
	if len(path) == directDependencyPathLength {
		key = path[directDependencyIndex].ComponentId
	}
	return key
}

// appendUniqueImpactPathsForMultipleRoots appends the source impact path to the target impact path while avoiding duplicates.
// Specifically, it is designed for handling multiple root projects, such as Maven or Gradle, by comparing each pair of paths and identifying the path that is closest to the direct dependency.
func appendUniqueImpactPathsForMultipleRoots(target [][]services.ImpactPathNode, source [][]services.ImpactPathNode) [][]services.ImpactPathNode {
	for targetPathIndex, targetPath := range target {
		for sourcePathIndex, sourcePath := range source {
			var subset []services.ImpactPathNode
			if len(sourcePath) <= len(targetPath) {
				subset = isImpactPathIsSubset(targetPath, sourcePath)
				if len(subset) != 0 {
					target[targetPathIndex] = subset
				}
			} else {
				subset = isImpactPathIsSubset(sourcePath, targetPath)
				if len(subset) != 0 {
					source[sourcePathIndex] = subset
				}
			}
		}
	}

	return appendUniqueImpactPaths(target, source, false)
}

// isImpactPathIsSubset checks if targetPath is a subset of sourcePath, and returns the subset if exists
func isImpactPathIsSubset(target []services.ImpactPathNode, source []services.ImpactPathNode) []services.ImpactPathNode {
	var subsetImpactPath []services.ImpactPathNode
	impactPathNodesMap := make(map[string]bool)
	for _, node := range target {
		impactPathNodesMap[node.ComponentId] = true
	}

	for _, node := range source {
		if impactPathNodesMap[node.ComponentId] {
			subsetImpactPath = append(subsetImpactPath, node)
		}
	}

	if len(subsetImpactPath) == len(target) || len(subsetImpactPath) == len(source) {
		return subsetImpactPath
	}
	return []services.ImpactPathNode{}
}

// appendUniqueFixVersions returns a new slice of strings that contains elements from both input slices without duplicates
func appendUniqueFixVersions(targetFixVersions []string, sourceFixVersions ...string) []string {
	fixVersionsSet := datastructures.MakeSet[string]()
	var result []string
	for _, fixVersion := range sourceFixVersions {
		fixVersionsSet.Add(fixVersion)
		result = append(result, fixVersion)
	}

	for _, fixVersion := range targetFixVersions {
		if exist := fixVersionsSet.Exists(fixVersion); !exist {
			result = append(result, fixVersion)
		}
	}
	return result
}

// GetUniqueKey returns a unique string key of format "vulnerableDependency:vulnerableVersion:xrayID:fixVersionExist"
func GetUniqueKey(vulnerableDependency, vulnerableVersion, xrayID string, fixVersionExist bool) string {
	return strings.Join([]string{vulnerableDependency, vulnerableVersion, xrayID, strconv.FormatBool(fixVersionExist)}, ":")
}

// If at least one cve is applicable - final value is applicable
// Else if at least one cve is undetermined - final value is undetermined
// Else (case when all cves aren't applicable) -> final value is not applicable
func getApplicableCveValue(extendedResults *ExtendedScanResults, xrayCves []formats.CveRow) string {
	if !extendedResults.EntitledForJas || len(extendedResults.ApplicabilityScanResults) == 0 {
		return ""
	}
	if len(xrayCves) == 0 {
		return ApplicabilityUndeterminedStringValue
	}
	cveExistsInResult := false
	finalApplicableValue := NotApplicableStringValue
	for _, cve := range xrayCves {
		if currentCveApplicableValue, exists := extendedResults.ApplicabilityScanResults[cve.Id]; exists {
			cveExistsInResult = true
			if currentCveApplicableValue == ApplicableStringValue {
				return currentCveApplicableValue
			} else if currentCveApplicableValue == ApplicabilityUndeterminedStringValue {
				finalApplicableValue = currentCveApplicableValue
			}
		}
	}
	if cveExistsInResult {
		return finalApplicableValue
	}
	return ApplicabilityUndeterminedStringValue
}

func printApplicableCveValue(applicableValue string, isTable bool) string {
	if applicableValue == ApplicableStringValue && isTable && (log.IsStdOutTerminal() && log.IsColorsSupported() ||
		os.Getenv("GITLAB_CI") != "") {
		return color.New(color.Red).Render(ApplicableStringValue)
	}
	return applicableValue
}
