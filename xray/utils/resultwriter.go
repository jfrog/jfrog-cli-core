package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type OutputFormat string

const (
	// OutputFormat values
	Table OutputFormat = "table"
	Json  OutputFormat = "json"
)

func PrintScanResults(results []services.ScanResponse, isTableFormat, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended bool) (err error) {
	if isTableFormat {
		var violations []services.Violation
		var vulnerabilities []services.Vulnerability
		var licenses []services.License
		for _, result := range results {
			violations = append(violations, result.Violations...)
			vulnerabilities = append(vulnerabilities, result.Vulnerabilities...)
			licenses = append(licenses, result.Licenses...)
		}

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
		if err != nil {
			return err
		}
	} else {
		err = printJson(results)
	}
	return err
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

func printJson(jsonRes []services.ScanResponse) error {
	results, err := json.Marshal(&jsonRes)
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
