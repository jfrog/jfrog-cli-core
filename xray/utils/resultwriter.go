package utils

import (
	"bytes"
	"encoding/json"
	"github.com/forPelevin/gomoji"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"strings"
)

type OutputFormat string

const (
	// OutputFormat values
	Table      OutputFormat = "table"
	Json       OutputFormat = "json"
	SimpleJson OutputFormat = "simple-json"
	Sarif      OutputFormat = "sarif"
)

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif)}

// Converting severities to their relevant score in range
var severitiesScore = map[string]string{
	"critical": "9", // Critical Score Range is 9.0 – 10.0
	"high":     "8", // High Score Range is 7.0 – 8.9
	"medium":   "6", // Medium Score Range is 4.0 – 6.9
	"low":      "3", // Low Score Range is 0.1 – 3.9
}

var dependenciesFiles = map[string]string{
	"go":     "go.mod",
	"maven":  "pom.xml",
	"npm":    "package.json",
	"nuget":  "NuGet Dependency File",
	"gradle": "Gradle Dependency File",
	"pip":    "Pip Dependency File",
	"pipenv": "Pipenv Dependency File",
	"python": "Python Dependency File",
}

// PrintScanResults prints Xray scan results in the given format.
// Note that errors are printed only on SimpleJson format.
func PrintScanResults(results []services.ScanResponse, errors []formats.SimpleJsonError, format OutputFormat, includeVulnerabilities, includeLicenses, isMultipleRoots, printExtended bool) error {
	switch format {
	case Table:
		var err error
		violations, vulnerabilities, licenses := splitScanResults(results)

		if len(results) > 0 {
			resultsPath, err := writeJsonResults(results)
			if err != nil {
				return err
			}
			log.Output("The full scan results are available here: " + resultsPath)
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
		jsonTable, err := convertScanToSimpleJson(results, errors, includeVulnerabilities, isMultipleRoots, includeLicenses)
		if err != nil {
			return err
		}
		return printJson(jsonTable)
	case Json:
		return printJson(results)
	case Sarif:
		sarifFile, err := GenerateSarifFileFromScan(results, includeVulnerabilities, isMultipleRoots)
		if err != nil {
			return err
		}
		log.Output(sarifFile)
	}
	return nil
}

func GenerateSarifFileFromScan(currentScan []services.ScanResponse, includeVulnerabilities, isMultipleRoots bool) (string, error) {
	// add check that scan has some vulnerabilities/violations
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	run := sarif.NewRunWithInformationURI("Xray", "https://jfrog.com/xray/")
	err = convertScanToSarif(run, currentScan, includeVulnerabilities, isMultipleRoots)
	if err != nil {
		return "", err
	}
	report.AddRun(run)
	out, err := json.Marshal(report)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return clientUtils.IndentJson(out), nil
}

func convertScanToSimpleJson(results []services.ScanResponse, errors []formats.SimpleJsonError, includeVulnerabilities, isMultipleRoots, includeLicenses bool) (formats.SimpleJsonResults, error) {
	violations, vulnerabilities, licenses := splitScanResults(results)
	jsonTable := formats.SimpleJsonResults{}
	if includeVulnerabilities {
		log.Info(noContextMessage + "All vulnerabilities detected will be included in the output JSON.")
		vulJsonTable, err := PrepareVulnerabilities(vulnerabilities, isMultipleRoots)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Vulnerabilities = vulJsonTable
	} else {
		secViolationsJsonTable, licViolationsJsonTable, opRiskViolationsJsonTable, err := PrepareViolations(violations, isMultipleRoots)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.SecurityViolations = secViolationsJsonTable
		jsonTable.LicensesViolations = licViolationsJsonTable
		jsonTable.OperationalRiskViolations = opRiskViolationsJsonTable
	}

	if includeLicenses {
		licJsonTable, err := PrepareLicenses(licenses, isMultipleRoots)
		if err != nil {
			return formats.SimpleJsonResults{}, err
		}
		jsonTable.Licenses = licJsonTable
	}
	jsonTable.Errors = errors

	return jsonTable, nil
}

func convertScanToSarif(run *sarif.Run, currentScan []services.ScanResponse, includeVulnerabilities, isMultipleRoots bool) error {
	var errors []formats.SimpleJsonError
	jsonTable, err := convertScanToSimpleJson(currentScan, errors, includeVulnerabilities, isMultipleRoots, false)
	if err != nil {
		return err
	}
	if len(jsonTable.SecurityViolations) > 0 {
		violations := jsonTable.SecurityViolations
		licenses := jsonTable.LicensesViolations
		for i := 0; i < len(jsonTable.SecurityViolations); i++ {
			impactedPackageFull := violations[i].ImpactedPackageName + " " + violations[i].ImpactedPackageVersion
			if violations[i].FixedVersions != nil {
				violations[i].Summary += "\n . Fixed in Versions: " + strings.Join(violations[i].FixedVersions, ",")
			}
			addScanResultsToSarifRun(run, strings.ToLower(gomoji.RemoveEmojis(violations[i].Severity)), violations[i].IssueId, impactedPackageFull, violations[i].Summary, violations[i].ImpactedPackageType)
		}
		for i := 0; i < len(licenses); i++ {
			impactedPackageFull := licenses[i].ImpactedPackageName + " " + licenses[i].ImpactedPackageVersion
			addScanResultsToSarifRun(run, strings.ToLower(gomoji.RemoveEmojis(licenses[i].Severity)), licenses[i].ImpactedPackageVersion, impactedPackageFull, licenses[i].LicenseKey, licenses[i].ImpactedPackageType)
		}
	} else if len(jsonTable.Vulnerabilities) > 0 {
		vulnerabilities := jsonTable.Vulnerabilities
		if err != nil {
			return err
		}
		for i := 0; i < len(vulnerabilities); i++ {
			impactedPackageFull := vulnerabilities[i].ImpactedPackageName + " " + vulnerabilities[i].ImpactedPackageVersion
			if vulnerabilities[i].FixedVersions != nil {
				vulnerabilities[i].Summary += "\n . Fixed in Versions: " + strings.Join(vulnerabilities[i].FixedVersions, ",")
			}
			addScanResultsToSarifRun(run, strings.ToLower(gomoji.RemoveEmojis(vulnerabilities[i].Severity)), vulnerabilities[i].IssueId, impactedPackageFull, vulnerabilities[i].Summary, vulnerabilities[i].ImpactedPackageType)
		}
	}

	return nil
}

func addScanResultsToSarifRun(run *sarif.Run, severity string, issueId string, impactedPackage string, description string, technology string) {
	path := dependenciesFiles[strings.ToLower(technology)]
	pb := sarif.NewPropertyBag()
	pb.Add("security-severity", severitiesScore[severity])
	run.AddRule(issueId).
		WithProperties(pb.Properties).
		WithFullDescription(sarif.NewMultiformatMessageString(description))
	run.CreateResultForRule(issueId).
		WithMessage(sarif.NewTextMessage(impactedPackage)).
		AddLocation(
			sarif.NewLocationWithPhysicalLocation(
				sarif.NewPhysicalLocation().
					WithArtifactLocation(
						sarif.NewSimpleArtifactLocation(path),
					),
			),
		)
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

func printJson(output interface{}) error {
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
	isEmpty := false
	if len(results) == 0 {
		isEmpty = true
	} else if len(results[0].Violations) == 0 &&
		len(results[0].Vulnerabilities) == 0 &&
		len(results[0].Licenses) == 0 {
		isEmpty = true
	}

	return isEmpty
}

func NewFailBuildError() error {
	return coreutils.CliError{ExitCode: coreutils.ExitCodeVulnerableBuild, ErrorMsg: "One or more of the violations found are set to fail builds that include them"}
}
