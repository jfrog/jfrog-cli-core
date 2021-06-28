package utils

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"strings"
)

// PrintViolationsTable prints the violations in 3 tables: security violations, license compliance violations and ignore rule URLs.
// In case one (or more) of the violations contains the field FailBuild set to true, CliError with exit code 3 will be returned.
func PrintViolationsTable(violations []services.Violation) error {
	securityViolationsTable := createTableWriter()
	securityViolationsTable.AppendHeader(table.Row{"Issue ID", "CVE", "CVSS v2", "CVSS v3", "Severity", "Component", "Version", "Fixed Versions", "Ignore Rule URL #"})
	licenseViolationsTable := createTableWriter()
	licenseViolationsTable.AppendHeader(table.Row{"License Key", "Severity", "Component", "Version", "Ignore Rule URL #"})
	ignoreUrlsTable := createTableWriter()
	ignoreUrlsTable.AppendHeader(table.Row{"#", "URL"})
	ignoreUrlCounter := 1 // Used to give a number to each ignore rule URL
	failBuild := false

	for _, violation := range violations {
		compNames, compVersions, compFixedVersions := splitComponents(violation.Components)
		if violation.ViolationType == "security" {
			cve, cvssV2, cvssV3 := splitCves(violation.Cves)
			for compIndex := 0; compIndex < len(compNames); compIndex++ {
				securityViolationsTable.AppendRow(table.Row{violation.IssueId, cve, cvssV2, cvssV3, violation.Severity, compNames[compIndex], compVersions[compIndex], compFixedVersions[compIndex], ignoreUrlCounter})
			}
		} else {
			// License compliance violation
			for compIndex := 0; compIndex < len(compNames); compIndex++ {
				licenseViolationsTable.AppendRow(table.Row{violation.LicenseKey, violation.Severity, compNames[compIndex], compVersions[compIndex], ignoreUrlCounter})
			}
		}
		ignoreUrlsTable.AppendRow(table.Row{ignoreUrlCounter, violation.IgnoreUrl})
		ignoreUrlCounter++
		if !failBuild && violation.FailBuild {
			failBuild = true
		}
	}

	// Print tables
	fmt.Println("SECURITY VIOLATIONS")
	securityViolationsTable.Render()
	fmt.Println("LICENSE COMPLIANCE VIOLATIONS")
	licenseViolationsTable.Render()
	fmt.Println("IGNORE RULES URLS")
	ignoreUrlsTable.Render()

	if failBuild {
		return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
	}

	return nil
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

func splitComponents(components map[string]services.Component) ([]string, []string, []string) {
	var compNames, compVersions, compFixedVersions []string
	for currCompId, currComp := range components {
		currCompName, currCompVersion := splitComponentId(currCompId)
		compNames = append(compNames, currCompName)
		compVersions = append(compVersions, currCompVersion)
		compFixedVersions = append(compFixedVersions, strings.Join(currComp.FixedVersions, "\n"))
	}
	return compNames, compVersions, compFixedVersions
}

func splitComponentId(componentId string) (string, string) {
	prefixSepIndex := strings.Index(componentId, "://") + 3
	trimmedComponentId := componentId[prefixSepIndex:]
	splitComponentId := strings.Split(trimmedComponentId, ":")
	return splitComponentId[len(splitComponentId)-2], splitComponentId[len(splitComponentId)-1]
}

func createTableWriter() table.Writer {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	tableWriter.SetStyle(table.StyleLight)
	tableWriter.Style().Options.SeparateRows = true
	return tableWriter
}
