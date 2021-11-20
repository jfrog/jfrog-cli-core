package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func PrintScanResults(results []services.ScanResponse, isTableFormat, includeVulnerabilities, includeLicenses, isMultipleRoots bool) (err error) {
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
			err = PrintVulnerabilitiesTable(vulnerabilities, isMultipleRoots)
		} else {
			err = PrintViolationsTable(violations, isMultipleRoots)
		}
		if err != nil {
			return err
		}
		if includeLicenses {
			err = PrintLicensesTable(licenses, isMultipleRoots)
		}
		if err != nil {
			return err
		}
	} else {
		err = printJson(results)
	}
	return err
}

func writeJsonResults(results []services.ScanResponse) (string, error) {
	out, err := fileutils.CreateTempFile()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	defer func() {
		e := out.Close()
		if err == nil {
			err = e
		}
	}()
	bytesRes, err := json.Marshal(&results)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, bytesRes, "", "  ")
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	_, err = out.Write([]byte(content.String()))
	return out.Name(), errorutils.CheckError(err)
}

func printJson(jsonRes []services.ScanResponse) error {
	results, err := json.Marshal(&jsonRes)
	if err != nil {
		return errorutils.CheckError(err)
	}
	fmt.Println(clientutils.IndentJson(results))
	return nil
}
