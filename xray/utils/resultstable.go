package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const noContextMessage = "Note: no context was provided, so no policy could be determined to scan against.\n" +
	"You can get a list of custom violations by providing one of the command options: --watches, --repo-path or --project.\n" +
	"Read more about configuring Xray policies here: https://www.jfrog.com/confluence/display/JFROG/Creating+Xray+Policies+and+Rules\n"

// PrintViolationsTable prints the violations in 3 tables: security violations, license compliance violations and ignore rule URLs.
// Set multipleRoots to true in case the given violations array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
// Set printExtended to true to print fields with 'extended' tag.
func PrintViolationsTable(violations []services.Violation, multipleRoots, printExtended bool) error {
	securityViolationsRows, licenseViolationsRows, err := PrepareViolationsTable(violations, multipleRoots, coreutils.IsTerminal())
	if err != nil {
		return err
	}

	// Print tables
	err = coreutils.PrintTable(securityViolationsRows, "Security Violations", "No security violations were found", printExtended)
	if err != nil {
		return err
	}
	return coreutils.PrintTable(licenseViolationsRows, "License Compliance Violations", "No license compliance violations were found", printExtended)
}

// Same as PrintViolationsTable, but table is returned as a json map array.
func CreateJsonViolationsTable(violations []services.Violation, multipleRoots bool) ([]map[string]interface{}, []map[string]interface{}, error) {
	securityViolationsRows, licenseViolationsRows, err := PrepareViolationsTable(violations, multipleRoots, false)
	if err != nil {
		return nil, nil, err
	}

	secViolationsJsonTable, err := coreutils.CreateJsonTable(securityViolationsRows)
	if err != nil {
		return nil, nil, err
	}
	licViolationsJsonTable, err := coreutils.CreateJsonTable(licenseViolationsRows)
	return secViolationsJsonTable, licViolationsJsonTable, err
}

func PrepareViolationsTable(violations []services.Violation, multipleRoots, coloredOutput bool) ([]vulnerabilityRow, []licenseViolationRow, error) {
	var securityViolationsRows []vulnerabilityRow
	var licenseViolationsRows []licenseViolationRow

	for _, violation := range violations {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, err := splitComponents(violation.Components, multipleRoots)
		if err != nil {
			return nil, nil, err
		}
		currSeverity := getSeverity(violation.Severity)
		if violation.ViolationType == "security" {
			cves := convertCves(violation.Cves)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				securityViolationsRows = append(securityViolationsRows,
					vulnerabilityRow{
						severity:               currSeverity.printableTitle(coloredOutput),
						severityNumValue:       currSeverity.numValue,
						impactedPackageName:    impactedPackagesNames[compIndex],
						impactedPackageVersion: impactedPackagesVersions[compIndex],
						impactedPackageType:    impactedPackagesTypes[compIndex],
						fixedVersions:          fixedVersions[compIndex],
						components:             components[compIndex],
						cves:                   cves,
						issueId:                violation.IssueId,
					},
				)
			}
		} else {
			// License compliance violation
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				licenseViolationsRows = append(licenseViolationsRows,
					licenseViolationRow{
						licenseKey:             violation.LicenseKey,
						severity:               currSeverity.printableTitle(coloredOutput),
						severityNumValue:       currSeverity.numValue,
						impactedPackageName:    impactedPackagesNames[compIndex],
						impactedPackageVersion: impactedPackagesVersions[compIndex],
						impactedPackageType:    impactedPackagesTypes[compIndex],
						components:             components[compIndex],
					},
				)
			}
		}
	}

	// Sort the rows by severity and whether the row contains fixed versions
	sort.Slice(securityViolationsRows, func(i, j int) bool {
		if securityViolationsRows[i].severityNumValue != securityViolationsRows[j].severityNumValue {
			return securityViolationsRows[i].severityNumValue > securityViolationsRows[j].severityNumValue
		}
		return securityViolationsRows[i].fixedVersions != "" && securityViolationsRows[j].fixedVersions == ""
	})
	sort.Slice(licenseViolationsRows, func(i, j int) bool {
		return licenseViolationsRows[i].severityNumValue > licenseViolationsRows[j].severityNumValue
	})

	return securityViolationsRows, licenseViolationsRows, nil
}

// PrintVulnerabilitiesTable prints the vulnerabilities in a table.
// Set multipleRoots to true in case the given vulnerabilities array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// Set printExtended to true to print fields with 'extended' tag.
func PrintVulnerabilitiesTable(vulnerabilities []services.Vulnerability, multipleRoots, printExtended bool) error {
	fmt.Println(noContextMessage + "Below are all vulnerabilities detected.")

	vulnerabilitiesRows, err := PrepareVulnerabilitiesTable(vulnerabilities, multipleRoots, coreutils.IsTerminal())
	if err != nil {
		return err
	}

	return coreutils.PrintTable(vulnerabilitiesRows, "Vulnerabilities", "No vulnerabilities were found", printExtended)
}

// Same as PrintVulnerabilitiesTable, but table is returned as a json map array.
func CreateJsonVulnerabilitiesTable(vulnerabilities []services.Vulnerability, multipleRoots bool) ([]map[string]interface{}, error) {
	vulnerabilitiesRows, err := PrepareVulnerabilitiesTable(vulnerabilities, multipleRoots, false)
	if err != nil {
		return nil, err
	}

	return coreutils.CreateJsonTable(vulnerabilitiesRows)
}

func PrepareVulnerabilitiesTable(vulnerabilities []services.Vulnerability, multipleRoots, coloredOutput bool) ([]vulnerabilityRow, error) {
	var vulnerabilitiesRows []vulnerabilityRow

	for _, vulnerability := range vulnerabilities {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions, components, err := splitComponents(vulnerability.Components, multipleRoots)
		if err != nil {
			return nil, err
		}
		cves := convertCves(vulnerability.Cves)
		currSeverity := getSeverity(vulnerability.Severity)
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			vulnerabilitiesRows = append(vulnerabilitiesRows,
				vulnerabilityRow{
					severity:               currSeverity.printableTitle(coloredOutput),
					severityNumValue:       currSeverity.numValue,
					impactedPackageName:    impactedPackagesNames[compIndex],
					impactedPackageVersion: impactedPackagesVersions[compIndex],
					impactedPackageType:    impactedPackagesTypes[compIndex],
					fixedVersions:          fixedVersions[compIndex],
					components:             components[compIndex],
					cves:                   cves,
					issueId:                vulnerability.IssueId,
				},
			)
		}
	}

	sort.Slice(vulnerabilitiesRows, func(i, j int) bool {
		if vulnerabilitiesRows[i].severityNumValue != vulnerabilitiesRows[j].severityNumValue {
			return vulnerabilitiesRows[i].severityNumValue > vulnerabilitiesRows[j].severityNumValue
		}
		return vulnerabilitiesRows[i].fixedVersions != "" && vulnerabilitiesRows[j].fixedVersions == ""
	})
	return vulnerabilitiesRows, nil
}

// PrintLicensesTable prints the licenses in a table.
// Set multipleRoots to true in case the given licenses array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleRoots is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// Set printExtended to true to print fields with 'extended' tag.
func PrintLicensesTable(licenses []services.License, multipleRoots, printExtended bool) error {
	licensesRows, err := PrepareJsonLicensesTable(licenses, multipleRoots)
	if err != nil {
		return err
	}

	return coreutils.PrintTable(licensesRows, "Licenses", "No licenses were found", printExtended)
}

// Same as PrintLicensesTable, but table is returned as a json map array.
func CreateJsonLicensesTable(licenses []services.License, multipleRoots bool) ([]map[string]interface{}, error) {
	licensesRows, err := PrepareJsonLicensesTable(licenses, multipleRoots)
	if err != nil {
		return nil, err
	}

	return coreutils.CreateJsonTable(licensesRows)
}

func PrepareJsonLicensesTable(licenses []services.License, multipleRoots bool) ([]licenseRow, error) {
	var licensesRows []licenseRow

	for _, license := range licenses {
		impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, _, components, err := splitComponents(license.Components, multipleRoots)
		if err != nil {
			return nil, err
		}
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			licensesRows = append(licensesRows,
				licenseRow{
					licenseKey:             license.Key,
					impactedPackageName:    impactedPackagesNames[compIndex],
					impactedPackageVersion: impactedPackagesVersions[compIndex],
					impactedPackageType:    impactedPackagesTypes[compIndex],
					components:             components[compIndex],
				},
			)
		}
	}

	return licensesRows, nil
}

// Used for vulnerabilities and security violations
type vulnerabilityRow struct {
	severity               string         `col-name:"Severity"`
	severityNumValue       int            // For sorting
	impactedPackageName    string         `col-name:"Impacted\nPackage"`
	impactedPackageVersion string         `col-name:"Impacted\nPackage\nVersion"`
	impactedPackageType    string         `col-name:"Type"`
	fixedVersions          string         `col-name:"Fixed\nVersions"`
	components             []componentRow `embed-table:"true"`
	cves                   []cveRow       `embed-table:"true"`
	issueId                string         `col-name:"Issue ID" extended:"true"`
}

type licenseRow struct {
	licenseKey             string         `col-name:"License"`
	impactedPackageName    string         `col-name:"Impacted\nPackage"`
	impactedPackageVersion string         `col-name:"Impacted\nPackage\nVersion"`
	impactedPackageType    string         `col-name:"Type"`
	components             []componentRow `embed-table:"true"`
}

type licenseViolationRow struct {
	licenseKey             string         `col-name:"License"`
	severity               string         `col-name:"Severity"`
	severityNumValue       int            // For sorting
	impactedPackageName    string         `col-name:"Impacted\nPackage"`
	impactedPackageVersion string         `col-name:"Impacted\nPackage\nVersion"`
	impactedPackageType    string         `col-name:"Type"`
	components             []componentRow `embed-table:"true"`
}

type componentRow struct {
	name    string `col-name:"Component"`
	version string `col-name:"Component\nVersion"`
}

type cveRow struct {
	id     string `col-name:"CVE"`
	cvssV2 string `col-name:"CVSS\nv2" extended:"true"`
	cvssV3 string `col-name:"CVSS\nv3" extended:"true"`
}

// This struct holds the sorted results of the simple-json output.
type ResultsSimpleJson struct {
	Vulnerabilities    []map[string]interface{}
	SecurityViolations []map[string]interface{}
	LicensesViolations []map[string]interface{}
	Licenses           []map[string]interface{}
}

func convertCves(cves []services.Cve) []cveRow {
	var cveRows []cveRow
	for _, cveObj := range cves {
		cveRows = append(cveRows, cveRow{id: cveObj.Id, cvssV2: cveObj.CvssV2Score, cvssV3: cveObj.CvssV3Score})
	}
	return cveRows
}

func splitComponents(impactedPackages map[string]services.Component, multipleRoots bool) ([]string, []string, []string, []string, [][]componentRow, error) {
	if len(impactedPackages) == 0 {
		return nil, nil, nil, nil, nil, errorutils.CheckErrorf("failed while parsing the response from Xray: violation doesn't have any components")
	}
	var impactedPackagesNames, impactedPackagesVersions, impactedPackagesTypes, fixedVersions []string
	var directComponents [][]componentRow
	for currCompId, currComp := range impactedPackages {
		currCompName, currCompVersion, currCompType := splitComponentId(currCompId)
		impactedPackagesNames = append(impactedPackagesNames, currCompName)
		impactedPackagesVersions = append(impactedPackagesVersions, currCompVersion)
		impactedPackagesTypes = append(impactedPackagesTypes, currCompType)
		fixedVersions = append(fixedVersions, strings.Join(currComp.FixedVersions, "\n"))
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

// splitComponentId splits an Xray component ID to the component name, version and package type.
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
		splitComponentId := strings.Split(packageId, ":")
		if len(splitComponentId) >= 3 {
			compName = splitComponentId[len(splitComponentId)-3]
			compVersion = fmt.Sprintf("%s:%s", splitComponentId[len(splitComponentId)-2], splitComponentId[len(splitComponentId)-1])
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
func getDirectComponents(impactPaths [][]services.ImpactPathNode, multipleRoots bool) []componentRow {
	var components []componentRow
	componentsMap := make(map[string]componentRow)

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
			componentsMap[componentId] = componentRow{name: compName, version: compVersion}
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
}

func (s *severity) printableTitle(colored bool) string {
	if !colored || len(s.style) == 0 {
		return s.title
	}
	return s.style.Render(s.title)
}

var severities = map[string]*severity{
	"Critical": {title: "Critical", numValue: 4, style: color.New(color.LightRed, color.Bold)},
	"High":     {title: "High", numValue: 3, style: color.New(color.Red, color.Bold)},
	"Medium":   {title: "Medium", numValue: 2, style: color.New(color.Yellow, color.Bold)},
	"Low":      {title: "Low", numValue: 1},
}

func getSeverity(severityTitle string) *severity {
	if severities[severityTitle] == nil {
		return &severity{title: severityTitle}
	}
	return severities[severityTitle]
}
