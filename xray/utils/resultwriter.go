package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type OutputFormat string

const (
	// OutputFormat values
	Table      OutputFormat = "table"
	Json       OutputFormat = "json"
	SimpleJson OutputFormat = "simple-json"
)

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson)}

func PrintScanResults(results []services.ScanResponse, format OutputFormat, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended bool) error {
	switch format {
	case Table:
		var err error
		violations, vulnerabilities, licenses := splitScanResults(results)

		if len(results) > 0 {
			resultsPath, err := writeJsonResults(results)
			if err != nil {
				return err
			}
			fmt.Println("The full scan results are available here: " + resultsPath)
		}
		if includeVulnerabilities {
			err = PrintVulnerabilitiesTable(vulnerabilities, isMultipleRoots, printExtended)
		} else {
			err = PrintViolationsTable(violations, isMultipleRoots, printExtended)
		}
		if err != nil {
			return err
		}
		if includeLicenses {
			err = PrintLicensesTable(licenses, isMultipleRoots, printExtended)
		}
		return err
	case SimpleJson:
		violations, vulnerabilities, licenses := splitScanResults(results)
		jsonTable := ResultsSimpleJson{}
		if includeVulnerabilities {
			log.Info(noContextMessage + "All vulnerabilities detected will be included in the output JSON.")
			vulJsonTable, err := CreateJsonVulnerabilitiesTable(vulnerabilities, isMultipleRoots)
			if err != nil {
				return err
			}
			jsonTable.Vulnerabilities = vulJsonTable
		} else {
			secViolationsJsonTable, licViolationsJsonTable, err := CreateJsonViolationsTable(violations, isMultipleRoots)
			if err != nil {
				return err
			}
			jsonTable.SecurityViolations = secViolationsJsonTable
			jsonTable.LicensesViolations = licViolationsJsonTable
		}

		if includeLicenses {
			licJsonTable, err := CreateJsonLicensesTable(licenses, isMultipleRoots)
			if err != nil {
				return err
			}
			jsonTable.Licenses = licJsonTable
		}
		return printJson(jsonTable)
	case Json:
		return printJson(results)
	}
	return nil
}

// Splits scan responses into aggregated lists of violations, vulnerabilities and licenses.
func splitScanResults(results []services.ScanResponse) ([]services.Violation, []services.Vulnerability, []services.License) {
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

func writeJsonResults(results []services.ScanResponse) (resultsPath string, err error) {
	out, err := fileutils.CreateTempFile()
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	defer func() {
		e := out.Close()
		if err == nil {
			err = e
		}
	}()
	bytesRes, err := json.Marshal(&results)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	var content bytes.Buffer
	err = json.Indent(&content, bytesRes, "", "  ")
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	_, err = out.Write(content.Bytes())
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	resultsPath = out.Name()
	return
}

func printJson(output interface{}) error {
	results, err := json.Marshal(output)
	if err != nil {
		return errorutils.CheckError(err)
	}
	fmt.Println(clientutils.IndentJson(results))
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

func NewFailBuildError() error {
	return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
}
