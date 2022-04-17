package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"sort"
	"strconv"
	"strings"

	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const noContextMessage = "Note: no context was provided, so no policy could be determined to scan against.\n" +
	"You can get a list of custom violations by providing one of the command options: --watches, --repo-path or --project.\n" +
	"Read more about configuring Xray policies here: https://www.jfrog.com/confluence/display/JFROG/Creating+Xray+Policies+and+Rules\n"

// PrintViolationsTable prints the violations in 4 tables: security violations, license compliance violations, operational risk violations and ignore rule URLs.
// Set multipleRoots to true in case the given violations array contains (or may contain) results of several projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
// Set printExtended to true to print fields with 'extended' tag.
func PrintViolationsTable(violations []services.Violation, multipleRoots, printExtended bool) error {
	securityViolationsRows, licenseViolationsRows, operationalRiskViolationsRows, err := PrepareViolations(violations, multipleRoots, coreutils.IsTerminal())
	if err != nil {
		return err
	}

	// Print tables
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
	return nil
}

func PrepareViolations(violations []services.Violation, multipleRoots, coloredOutput bool) ([]formats.VulnerabilityOrViolationRow, []formats.LicenseViolationRow, []formats.OperationalRiskViolationRow, error) {
	var securityViolationsRows []formats.VulnerabilityOrViolationRow
	var licenseViolationsRows []formats.LicenseViolationRow
	var operationalRiskViolationsRows []formats.OperationalRiskViolationRow

	for _, violation := range violations {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, err := splitComponents(violation.Components, multipleRoots)
		if err != nil {
			return nil, nil, nil, err
		}
		currSeverity := getSeverity(violation.Severity)
		switch violation.ViolationType {
		case "security":
			cves := convertCves(violation.Cves)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				securityViolationsRows = append(securityViolationsRows,
					formats.VulnerabilityOrViolationRow{
						Summary:                violation.Summary,
						Severity:               currSeverity.printableTitle(coloredOutput),
						SeverityNumValue:       currSeverity.numValue,
						ImpactedPackageName:    impactedPackagesNames[compIndex],
						ImpactedPackageVersion: impactedPackagesVersions[compIndex],
						ImpactedPackageType:    impactedPackagesTypes[compIndex],
						FixedVersions:          fixedVersions[compIndex],
						Components:             components[compIndex],
						Cves:                   cves,
						IssueId:                violation.IssueId,
						References:             violation.References,
					},
				)
			}
		case "license":
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				licenseViolationsRows = append(licenseViolationsRows,
					formats.LicenseViolationRow{
						LicenseKey:             violation.LicenseKey,
						Severity:               currSeverity.printableTitle(coloredOutput),
						SeverityNumValue:       currSeverity.numValue,
						ImpactedPackageName:    impactedPackagesNames[compIndex],
						ImpactedPackageVersion: impactedPackagesVersions[compIndex],
						ImpactedPackageType:    impactedPackagesTypes[compIndex],
						Components:             components[compIndex],
					},
				)
			}
		case "operational_risk":
			violationOpRiskData := getOperationalRiskViolationReadableData(violation)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				operationalRiskViolationsRow := &formats.OperationalRiskViolationRow{
					Severity:               currSeverity.printableTitle(coloredOutput),
					SeverityNumValue:       currSeverity.numValue,
					ImpactedPackageName:    impactedPackagesNames[compIndex],
					ImpactedPackageVersion: impactedPackagesVersions[compIndex],
					ImpactedPackageType:    impactedPackagesTypes[compIndex],
					Components:             components[compIndex],
					IsEol:                  violationOpRiskData.isEol,
					Cadence:                violationOpRiskData.cadence,
					Commits:                violationOpRiskData.commits,
					Committers:             violationOpRiskData.committers,
					NewerVersions:          violationOpRiskData.newerVersions,
					LatestVersion:          violationOpRiskData.latestVersion,
					RiskReason:             violationOpRiskData.riskReason,
					EolMessage:             violationOpRiskData.eolMessage,
				}
				operationalRiskViolationsRows = append(operationalRiskViolationsRows, *operationalRiskViolationsRow)
			}
		default:
			// Unsupported type, ignore
		}
	}

	// Sort the rows by severity and whether the row contains fixed versions
	sort.Slice(securityViolationsRows, func(i, j int) bool {
		if securityViolationsRows[i].SeverityNumValue != securityViolationsRows[j].SeverityNumValue {
			return securityViolationsRows[i].SeverityNumValue > securityViolationsRows[j].SeverityNumValue
		}
		return len(securityViolationsRows[i].FixedVersions) > 0 && len(securityViolationsRows[j].FixedVersions) > 0
	})
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
func PrintVulnerabilitiesTable(vulnerabilities []services.Vulnerability, multipleRoots, printExtended bool) error {
	fmt.Println(noContextMessage + "Below are all vulnerabilities detected.")

	vulnerabilitiesRows, err := PrepareVulnerabilities(vulnerabilities, multipleRoots, coreutils.IsTerminal())
	if err != nil {
		return err
	}

	return coreutils.PrintTable(formats.ConvertToVulnerabilityTableRow(vulnerabilitiesRows), "Vulnerabilities", "✨ No vulnerabilities were found ✨", printExtended)
}

func PrepareVulnerabilities(vulnerabilities []services.Vulnerability, multipleRoots, coloredOutput bool) ([]formats.VulnerabilityOrViolationRow, error) {
	var vulnerabilitiesRows []formats.VulnerabilityOrViolationRow

	for _, vulnerability := range vulnerabilities {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, err := splitComponents(vulnerability.Components, multipleRoots)
		if err != nil {
			return nil, err
		}
		cves := convertCves(vulnerability.Cves)
		currSeverity := getSeverity(vulnerability.Severity)
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			vulnerabilitiesRows = append(vulnerabilitiesRows,
				formats.VulnerabilityOrViolationRow{
					Summary:                vulnerability.Summary,
					Severity:               currSeverity.printableTitle(coloredOutput),
					SeverityNumValue:       currSeverity.numValue,
					ImpactedPackageName:    impactedPackagesNames[compIndex],
					ImpactedPackageVersion: impactedPackagesVersions[compIndex],
					ImpactedPackageType:    impactedPackagesTypes[compIndex],
					FixedVersions:          fixedVersions[compIndex],
					Components:             components[compIndex],
					Cves:                   cves,
					IssueId:                vulnerability.IssueId,
					References:             vulnerability.References,
				},
			)
		}
	}

	sort.Slice(vulnerabilitiesRows, func(i, j int) bool {
		if vulnerabilitiesRows[i].SeverityNumValue != vulnerabilitiesRows[j].SeverityNumValue {
			return vulnerabilitiesRows[i].SeverityNumValue > vulnerabilitiesRows[j].SeverityNumValue
		}
		return len(vulnerabilitiesRows[i].FixedVersions) > 0 && len(vulnerabilitiesRows[j].FixedVersions) > 0
	})
	return vulnerabilitiesRows, nil
}

// PrintLicensesTable prints the licenses in a table.
// Set multipleRoots to true in case the given licenses array contains (or may contain) results of several projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// Set printExtended to true to print fields with 'extended' tag.
func PrintLicensesTable(licenses []services.License, multipleRoots, printExtended bool) error {
	licensesRows, err := PrepareLicenses(licenses, multipleRoots)
	if err != nil {
		return err
	}

	return coreutils.PrintTable(formats.ConvertToLicenseTableRow(licensesRows), "Licenses", "No licenses were found", printExtended)
}

func PrepareLicenses(licenses []services.License, multipleRoots bool) ([]formats.LicenseRow, error) {
	var licensesRows []formats.LicenseRow

	for _, license := range licenses {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, _, components, err := splitComponents(license.Components, multipleRoots)
		if err != nil {
			return nil, err
		}
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			licensesRows = append(licensesRows,
				formats.LicenseRow{
					LicenseKey:             license.Key,
					ImpactedPackageName:    impactedPackagesNames[compIndex],
					ImpactedPackageVersion: impactedPackagesVersions[compIndex],
					ImpactedPackageType:    impactedPackagesTypes[compIndex],
					Components:             components[compIndex],
				},
			)
		}
	}

	return licensesRows, nil
}

func convertCves(cves []services.Cve) []formats.CveRow {
	var cveRows []formats.CveRow
	for _, cveObj := range cves {
		cveRows = append(cveRows, formats.CveRow{Id: cveObj.Id, CvssV2: cveObj.CvssV2Score, CvssV3: cveObj.CvssV3Score})
	}
	return cveRows
}

func splitComponents(impactedPackages map[string]services.Component, multipleRoots bool) ([]string, []string, []string, [][]string, [][]formats.ComponentRow, error) {
	if len(impactedPackages) == 0 {
		return nil, nil, nil, nil, nil, errorutils.CheckErrorf("failed while parsing the response from Xray: violation doesn't have any components")
	}
	var impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes []string
	var fixedVersions [][]string
	var directComponents [][]formats.ComponentRow
	for currCompId, currComp := range impactedPackages {
		currCompName, currCompVersion, currCompType := splitComponentId(currCompId)
		impactedPackagesNames = append(impactedPackagesNames, currCompName)
		impactedPackagesVersions = append(impactedPackagesVersions, currCompVersion)
		impactedPackagesTypes = append(impactedPackagesTypes, currCompType)
		fixedVersions = append(fixedVersions, currComp.FixedVersions)
		currComponents := getDirectComponents(currComp.ImpactPaths, multipleRoots)
		directComponents = append(directComponents, currComponents)
	}
	return impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, directComponents, nil
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

// splitComponentId splits a Xray component ID to the component name, version and package type.
// In case componentId doesn't contain a version, the returned version will be an empty string.
// In case componentId's format is invalid, it will be returned as the component name
// and empty strings will be returned instead of the version and the package type.
// Examples:
// 1. componentId: "gav://antparent:ant:1.6.5"
//    Returned values:
//      Component name: "antparent:ant"
//      Component version: "1.6.5"
//      Package type: "Maven"
// 2. componentId: "generic://sha256:244fd47e07d1004f0aed9c156aa09083c82bf8944eceb67c946ff7430510a77b/foo.jar"
//    Returned values:
//      Component name: "foo.jar"
//      Component version: ""
//      Package type: "Generic"
// 3. componentId: "invalid-comp-id"
//    Returned values:
//      Component name: "invalid-comp-id"
//      Component version: ""
//      Package type: ""
func splitComponentId(componentId string) (string, string, string) {
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

// Gets a string of the direct dependencies or packages of the scanned component, that depends on the vulnerable package
func getDirectComponents(impactPaths [][]services.ImpactPathNode, multipleRoots bool) []formats.ComponentRow {
	var components []formats.ComponentRow
	componentsMap := make(map[string]formats.ComponentRow)

	// The first node in the impact path is the scanned component itself. The second one is the direct dependency.
	impactPathLevel := 1
	if multipleRoots {
		impactPathLevel = 0
	}

	for _, impactPath := range impactPaths {
		impactPathIndex := impactPathLevel
		if len(impactPath) <= impactPathLevel {
			impactPathIndex = len(impactPath) - 1
		}
		componentId := impactPath[impactPathIndex].ComponentId
		if _, exist := componentsMap[componentId]; !exist {
			compName, compVersion, _ := splitComponentId(componentId)
			componentsMap[componentId] = formats.ComponentRow{Name: compName, Version: compVersion}
		}
	}

	for _, row := range componentsMap {
		components = append(components, row)
	}
	return components
}

type severity struct {
	title    string
	numValue int
	style    color.Style
	emoji    string
}

func (s *severity) printableTitle(colored bool) string {
	if !colored {
		return s.title
	}
	if len(s.style) == 0 {
		return s.emoji + s.title
	}
	return s.style.Render(s.emoji + s.title)
}

var severities = map[string]*severity{
	"Critical": {emoji: "💀", title: "Critical", numValue: 4, style: color.New(color.BgLightRed, color.LightWhite)},
	"High":     {emoji: "🔥", title: "High", numValue: 3, style: color.New(color.Red)},
	"Medium":   {emoji: "🎃", title: "Medium", numValue: 2, style: color.New(color.Yellow)},
	"Low":      {emoji: "👻", title: "Low", numValue: 1},
}

func getSeverity(severityTitle string) *severity {
	if severities[severityTitle] == nil {
		return &severity{title: severityTitle}
	}
	return severities[severityTitle]
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
