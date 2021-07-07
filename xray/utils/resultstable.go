package utils

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"sort"
	"strings"
)

// PrintViolationsTable prints the violations in 3 tables: security violations, license compliance violations and ignore rule URLs.
// Set multipleComponents to true in case the given violations array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleComponents is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
func PrintViolationsTable(violations []services.Violation, multipleComponents bool) error {
	securityViolationsTable := createTableWriter()
	// Temporarily removed ignore rule URL column
	//securityViolationsTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Impacted Package", "Impacted Package\nVersion", "Fixed Versions", "Component", "Component\nVersion", "Ignore Rule URL #"})
	securityViolationsTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Impacted Package", "Impacted Package\nVersion", "Fixed Versions", "Component", "Component\nVersion"})
	licenseViolationsTable := createTableWriter()
	// Temporarily removed ignore rule URL column
	//licenseViolationsTable.AppendHeader(table.Row{"License", "Severity", "Impacted Package", "Impacted Package\nVersion", "Component", "Component\nVersion", "Ignore Rule URL #"})
	licenseViolationsTable.AppendHeader(table.Row{"License", "Severity", "Impacted Package", "Impacted Package\nVersion", "Component", "Component\nVersion"})
	// Temporarily removed
	//ignoreUrlsTable := createTableWriter()
	//ignoreUrlsTable.AppendHeader(table.Row{"#", "URL"})
	//ignoreUrlCounter := 1 // Used to give a number to each ignore rule URL
	failBuild := false

	sort.Slice(violations, func(i, j int) bool {
		return compareSeverity(violations[i].Severity, violations[j].Severity)
	})

	isTerminal := terminal.IsTerminal(int(os.Stderr.Fd()))

	for _, violation := range violations {
		impactedPackagesNames, impactedPackagesVersions, fixedVersions, compNames, compVersions := splitComponents(violation.Components, multipleComponents)
		if violation.ViolationType == "security" {
			cve, cvssV2, cvssV3 := splitCves(violation.Cves)
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				// Temporarily removed ignore rule URL column
				//securityViolationsTable.AppendRow(table.Row{violation.IssueId, cve, cvssV2, cvssV3, getSeverity(violation.Severity).printableTitle(isTerminal), impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], fixedVersions[compIndex], compNames[compIndex], compVersions[compIndex], ignoreUrlCounter})
				securityViolationsTable.AppendRow(table.Row{violation.IssueId, cve, cvssV2, cvssV3, getSeverity(violation.Severity).printableTitle(isTerminal), impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], fixedVersions[compIndex], compNames[compIndex], compVersions[compIndex]})
			}
		} else {
			// License compliance violation
			for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
				// Temporarily removed ignore rule URL column
				//licenseViolationsTable.AppendRow(table.Row{violation.LicenseKey, getSeverity(violation.Severity).printableTitle(isTerminal), impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], compNames[compIndex], compVersions[compIndex], ignoreUrlCounter})
				licenseViolationsTable.AppendRow(table.Row{violation.LicenseKey, getSeverity(violation.Severity).printableTitle(isTerminal), impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], compNames[compIndex], compVersions[compIndex]})
			}
		}
		// Temporarily removed
		//ignoreUrlsTable.AppendRow(table.Row{ignoreUrlCounter, violation.IgnoreUrl})
		//ignoreUrlCounter++
		if !failBuild && violation.FailBuild {
			failBuild = true
		}
	}

	// Print tables
	printTable("Security Violations", securityViolationsTable, true)
	printTable("License Compliance Violations", licenseViolationsTable, true)
	// Temporarily removed
	//printTable("Ignore Rules URLs", ignoreUrlsTable, false)

	if failBuild {
		return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
	}

	return nil
}

// PrintVulnerabilitiesTable prints the vulnerabilities in a table.
// Set multipleComponents to true in case the given vulnerabilities array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleComponents is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
func PrintVulnerabilitiesTable(vulnerabilities []services.Vulnerability, multipleComponents bool) {
	fmt.Println("Note: no context was provided, so no policy could be determined to scan against.\n" +
		"You can get a list of custom violations by providing one of the command options: --watches, --target-path or --project.\n" +
		"Read more about configuring Xray policies here: https://www.jfrog.com/confluence/display/JFROG/Creating+Xray+Policies+and+Rules\n" +
		"Below are all vulnerabilities detected.")
	vulnerabilitiesTable := createTableWriter()
	vulnerabilitiesTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Impacted Package", "Impacted Package\nVersion", "Fixed Versions", "Component", "Component\nVersion"})

	sort.Slice(vulnerabilities, func(i, j int) bool {
		return compareSeverity(vulnerabilities[i].Severity, vulnerabilities[j].Severity)
	})

	isTerminal := terminal.IsTerminal(int(os.Stderr.Fd()))

	for _, vulnerability := range vulnerabilities {
		impactedPackagesNames, impactedPackagesVersions, fixedVersions, compNames, compVersions := splitComponents(vulnerability.Components, multipleComponents)
		cve, cvssV2, cvssV3 := splitCves(vulnerability.Cves)
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			vulnerabilitiesTable.AppendRow(table.Row{vulnerability.IssueId, cve, cvssV2, cvssV3, getSeverity(vulnerability.Severity).printableTitle(isTerminal), impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], fixedVersions[compIndex], compNames[compIndex], compVersions[compIndex]})
		}
	}

	printTable("Vulnerabilities", vulnerabilitiesTable, true)
}

// PrintLicensesTable prints the licenses in a table.
// Set multipleComponents to true in case the given licenses array contains (or may contain) results of several different projects or files (like in binary scan).
// In case multipleComponents is true, the field Component will show the root of each impact path, otherwise it will show the root's child.
func PrintLicensesTable(licenses []services.License, multipleComponents bool) {
	licensesTable := createTableWriter()
	licensesTable.AppendHeader(table.Row{"License", "Impacted Package", "Impacted Package\nVersion", "Component", "Component\nVersion"})

	for _, license := range licenses {
		impactedPackagesNames, impactedPackagesVersions, _, compNames, compVersions := splitComponents(license.Components, multipleComponents)
		for compIndex := 0; compIndex < len(impactedPackagesNames); compIndex++ {
			licensesTable.AppendRow(table.Row{license.Key, impactedPackagesNames[compIndex], impactedPackagesVersions[compIndex], compNames[compIndex], compVersions[compIndex]})
		}
	}

	printTable("Licenses", licensesTable, true)
}

func splitCves(cves []services.Cve) (string, string, string) {
	var cve, cvssV2, cvssV3 string
	if len(cves) == 0 {
		return "", "", ""
	}
	for _, cveObj := range cves {
		cve = fmt.Sprintf("%s%s\n", cve, cveObj.Id)
		cvssV2 = fmt.Sprintf("%s%s\n", cvssV2, cveObj.CvssV2Score)
		cvssV3 = fmt.Sprintf("%s%s\n", cvssV3, cveObj.CvssV3Score)
	}
	return cve[:len(cve)-1], cvssV2[:len(cvssV2)-1], cvssV3[:len(cvssV3)-1]
}

func splitComponents(components map[string]services.Component, multipleRoots bool) ([]string, []string, []string, []string, []string) {
	var impactedPackagesNames, impactedPackagesVersions, fixedVersions, compNames, compVersions []string
	for currCompId, currComp := range components {
		currCompName, currCompVersion := splitComponentId(currCompId)
		impactedPackagesNames = append(impactedPackagesNames, currCompName)
		impactedPackagesVersions = append(impactedPackagesVersions, currCompVersion)
		fixedVersions = append(fixedVersions, strings.Join(currComp.FixedVersions, "\n"))
		currCompNames, currCompVersions := getDirectComponents(currComp.ImpactPaths, multipleRoots)
		compNames = append(compNames, currCompNames)
		compVersions = append(compVersions, currCompVersions)
	}
	return impactedPackagesNames, impactedPackagesVersions, fixedVersions, compNames, compVersions
}

func splitComponentId(componentId string) (string, string) {
	compIdParts := strings.Split(componentId, "://")
	// Invalid component ID
	if len(compIdParts) != 2 {
		return componentId, ""
	}

	packageType := compIdParts[0]
	packageId := compIdParts[1]

	// Generic identifier structure: generic://sha256:<Checksum>/name
	if packageType == "generic" {
		lastSlashIndex := strings.LastIndex(packageId, "/")
		return packageId[lastSlashIndex+1:], ""
	}

	splitComponentId := strings.Split(packageId, ":")

	var compName, compVersion string
	switch packageType {
	case "rpm":
		// RPM identifier structure: rpm://os-version:package:epoch-version:version
		// os-version is optional.
		if len(splitComponentId) >= 3 {
			compName = splitComponentId[len(splitComponentId)-3]
			compVersion = splitComponentId[len(splitComponentId)-1]
		}
	default:
		// All other identifiers look like this: package-type://package-name:version.
		// Sometimes there's a namespace or a group before the package name, separated by a '/' or a ':'.
		if len(splitComponentId) >= 2 {
			compName = splitComponentId[len(splitComponentId)-2]
			compVersion = splitComponentId[len(splitComponentId)-1]
		}
	}

	// If there's an error while parsing the component ID
	if compName == "" {
		compName = packageId
	}

	return compName, compVersion
}

// Gets a string of the direct dependencies or packages of the scanned component, that depends on the vulnerable package
func getDirectComponents(impactPaths [][]services.ImpactPathNode, multipleRoots bool) (string, string) {
	var compNames, compVersions string

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
		compName, compVersion := splitComponentId(impactPath[impactPathIndex].ComponentId)
		compNames = fmt.Sprintf("%s%s\n", compNames, compName)
		compVersions = fmt.Sprintf("%s%s\n", compVersions, compVersion)
	}
	return compNames[:len(compNames)-1], compVersions[:len(compVersions)-1]
}

func createTableWriter() table.Writer {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	tableWriter.SetStyle(table.StyleLight)
	tableWriter.Style().Options.SeparateRows = true
	return tableWriter
}

func printTable(pluralEntityName string, tableWriter table.Writer, printMessageIfEmpty bool) {
	if tableWriter.Length() == 0 {
		if printMessageIfEmpty {
			fmt.Println(strings.Title(pluralEntityName))
			printMessage(fmt.Sprintf("No %s found", strings.ToLower(pluralEntityName)))
		}
		return
	}

	fmt.Println(pluralEntityName)
	tableWriter.Render()
}

func printMessage(message string) {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	tableWriter.SetStyle(table.StyleLight)
	tableWriter.AppendRow(table.Row{message})
	tableWriter.Render()
}

type severity struct {
	title    string
	numValue int
	style    color.Style
}

func (s *severity) printableTitle(colorful bool) string {
	if !colorful || len(s.style) == 0 {
		return s.title
	}
	return s.style.Render(s.title)
}

var severities = map[string]*severity{
	"Critical": {title: "Critical", numValue: 4, style: color.New(color.LightRed, color.Bold)},
	"High": {title: "High", numValue: 3, style: color.New(color.Red, color.Bold)},
	"Medium": {title: "Medium", numValue: 2, style: color.New(color.Yellow, color.Bold)},
	"Low": {title: "Low", numValue: 1},
}

func getSeverity(severityTitle string) *severity {
	if severities[severityTitle] == nil {
		return &severity{title: severityTitle}
	}
	return severities[severityTitle]
}

func compareSeverity(severityA, severityB string) bool {
	valueA := getSeverity(severityA)
	valueB := getSeverity(severityB)
	return valueA.numValue > valueB.numValue
}
