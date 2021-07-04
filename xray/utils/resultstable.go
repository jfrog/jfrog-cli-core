package utils

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"strings"
)

// PrintViolationsTable prints the violations in 3 tables: security violations, license compliance violations and ignore rule URLs.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
func PrintViolationsTable(violations []services.Violation) error {
	securityViolationsTable := createTableWriter()
	// Temporarily removed ignore rule URL column
	//securityViolationsTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Component", "Version", "Fixed Versions", "Direct Components", "Ignore Rule URL #"})
	securityViolationsTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Component", "Version", "Fixed Versions", "Direct Components"})
	licenseViolationsTable := createTableWriter()
	// Temporarily removed ignore rule URL column
	//licenseViolationsTable.AppendHeader(table.Row{"License Key", "Severity", "Component", "Version", "Direct Components", "Ignore Rule URL #"})
	licenseViolationsTable.AppendHeader(table.Row{"License Key", "Severity", "Component", "Version", "Direct Components"})
	// Temporarily removed
	//ignoreUrlsTable := createTableWriter()
	//ignoreUrlsTable.AppendHeader(table.Row{"#", "URL"})
	//ignoreUrlCounter := 1 // Used to give a number to each ignore rule URL
	failBuild := false

	for _, violation := range violations {
		compNames, compVersions, compFixedVersions, directComponents := splitComponents(violation.Components)
		if violation.ViolationType == "security" {
			cve, cvssV2, cvssV3 := splitCves(violation.Cves)
			for compIndex := 0; compIndex < len(compNames); compIndex++ {
				// Temporarily removed ignore rule URL column
				//securityViolationsTable.AppendRow(table.Row{violation.IssueId, cve, cvssV2, cvssV3, violation.Severity, compNames[compIndex], compVersions[compIndex], compFixedVersions[compIndex], directComponents[compIndex], ignoreUrlCounter})
				securityViolationsTable.AppendRow(table.Row{violation.IssueId, cve, cvssV2, cvssV3, violation.Severity, compNames[compIndex], compVersions[compIndex], compFixedVersions[compIndex], directComponents[compIndex]})
			}
		} else {
			// License compliance violation
			for compIndex := 0; compIndex < len(compNames); compIndex++ {
				// Temporarily removed ignore rule URL column
				//licenseViolationsTable.AppendRow(table.Row{violation.LicenseKey, violation.Severity, compNames[compIndex], compVersions[compIndex], directComponents[compIndex], ignoreUrlCounter})
				licenseViolationsTable.AppendRow(table.Row{violation.LicenseKey, violation.Severity, compNames[compIndex], compVersions[compIndex], directComponents[compIndex]})
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
	fmt.Println("SECURITY VIOLATIONS")
	securityViolationsTable.Render()
	fmt.Println("LICENSE COMPLIANCE VIOLATIONS")
	licenseViolationsTable.Render()
	// Temporarily removed
	//fmt.Println("IGNORE RULES URLS")
	//ignoreUrlsTable.Render()

	if failBuild {
		return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
	}

	return nil
}

func PrintVulnerabilitiesTable(vulnerabilities []services.Vulnerability) {
	fmt.Println("Note: no context was provided (--watches, --target-path or --project), so no policy could be determined to scan against. Below are all vulnerabilities detected.")
	vulnerabilitiesTable := createTableWriter()
	vulnerabilitiesTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Component", "Version", "Fixed Versions", "Direct Components"})

	for _, vulnerability := range vulnerabilities {
		compNames, compVersions, compFixedVersions, directComponents := splitComponents(vulnerability.Components)
		cve, cvssV2, cvssV3 := splitCves(vulnerability.Cves)
		for compIndex := 0; compIndex < len(compNames); compIndex++ {
			vulnerabilitiesTable.AppendRow(table.Row{vulnerability.IssueId, cve, cvssV2, cvssV3, vulnerability.Severity, compNames[compIndex], compVersions[compIndex], compFixedVersions[compIndex], directComponents[compIndex]})
		}
	}

	fmt.Println("VULNERABILITIES")
	vulnerabilitiesTable.Render()
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

func splitComponents(components map[string]services.Component) ([]string, []string, []string, []string) {
	var compNames, compVersions, compFixedVersions, directComponents []string
	for currCompId, currComp := range components {
		currCompName, currCompVersion := splitComponentId(currCompId)
		compNames = append(compNames, currCompName)
		compVersions = append(compVersions, currCompVersion)
		compFixedVersions = append(compFixedVersions, strings.Join(currComp.FixedVersions, "\n"))
		directComponents = append(directComponents, getDirectComponents(currComp.ImpactPaths))
	}
	return compNames, compVersions, compFixedVersions, directComponents
}

func splitComponentId(componentId string) (string, string) {
	prefixSepIndex := strings.Index(componentId, "://")
	packageType := componentId[:prefixSepIndex]

	lastSlashIndex := strings.LastIndex(componentId, "/")
	trimmedComponentId := componentId[lastSlashIndex+1:]
	splitComponentId := strings.Split(trimmedComponentId, ":")

	var compName, compVersion string
	switch packageType {
	case "rpm":
		// RPM identifier structure: rpm://os-version:package:epoch-version:version
		compName = splitComponentId[1]
		compVersion = splitComponentId[3]
	case "generic":
		// Generic identifier structure: generic://sha256:<Checksum>/name
		compName = splitComponentId[0]
	default:
		// All other identifiers look like this: package-type://package-name:version.
		// Sometimes there's a namespace or a group before the package name, separated by a '/' or a ':'.
		compName = splitComponentId[len(splitComponentId)-2]
		compVersion = splitComponentId[len(splitComponentId)-1]
	}

	return compName, compVersion
}

// Gets a string of the direct dependencies of the scanned component, that depends on the vulnerable component
func getDirectComponents(impactPaths [][]services.ImpactPathNode) string {
	var directComponentsStr string
	for _, impactPath := range impactPaths {
		// The first node in the impact path is the scanned component itself, so the second one is the direct dependency
		compName, _ := splitComponentId(impactPath[1].ComponentId)
		directComponentsStr = fmt.Sprintf("%s%s\n", directComponentsStr, compName)
	}
	return directComponentsStr[:len(directComponentsStr)-1]
}

func createTableWriter() table.Writer {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	tableWriter.SetStyle(table.StyleLight)
	tableWriter.Style().Options.SeparateRows = true
	return tableWriter
}
