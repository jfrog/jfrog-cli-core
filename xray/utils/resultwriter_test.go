package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
)

func TestGetVulnerabilityOrViolationSarifHeadline(t *testing.T) {
	assert.Equal(t, "[CVE-2022-1234] loadsh 1.4.1", getXrayIssueSarifHeadline("loadsh", "1.4.1", "CVE-2022-1234"))
	assert.NotEqual(t, "[CVE-2022-1234] loadsh 1.4.1", getXrayIssueSarifHeadline("loadsh", "1.2.1", "CVE-2022-1234"))
}

func TestGetIssueIdentifier(t *testing.T) {
	issueId := "XRAY-123456"
	cvesRow := []formats.CveRow{{Id: "CVE-2022-1234"}}
	assert.Equal(t, "CVE-2022-1234", GetIssueIdentifier(cvesRow, issueId))
	cvesRow = append(cvesRow, formats.CveRow{Id: "CVE-2019-1234"})
	assert.Equal(t, "CVE-2022-1234, CVE-2019-1234", GetIssueIdentifier(cvesRow, issueId))
	assert.Equal(t, issueId, GetIssueIdentifier(nil, issueId))
}

func TestGetDirectDependenciesFormatted(t *testing.T) {
	testCases := []struct {
		name           string
		directDeps     []formats.ComponentRow
		expectedOutput string
	}{
		{
			name: "Single direct dependency",
			directDeps: []formats.ComponentRow{
				{Name: "example-package", Version: "1.0.0"},
			},
			expectedOutput: "`example-package 1.0.0`",
		},
		{
			name: "Multiple direct dependencies",
			directDeps: []formats.ComponentRow{
				{Name: "dependency1", Version: "1.0.0"},
				{Name: "dependency2", Version: "2.0.0"},
			},
			expectedOutput: "`dependency1 1.0.0`<br/>`dependency2 2.0.0`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := getDirectDependenciesFormatted(tc.directDeps)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}

func TestGetSarifTableDescription(t *testing.T) {
	testCases := []struct {
		name                string
		formattedDeps       string
		maxCveScore         string
		applicable          string
		fixedVersions       []string
		expectedDescription string
	}{
		{
			name:                "Applicable vulnerability",
			formattedDeps:       "`example-package 1.0.0`",
			maxCveScore:         "7.5",
			applicable:          "Applicable",
			fixedVersions:       []string{"1.0.1", "1.0.2"},
			expectedDescription: "| Severity Score | Contextual Analysis | Direct Dependencies | Fixed Versions     |\n|  :---:  |  :---:  |  :---:  |  :---:  |\n| 7.5      | Applicable       | `example-package 1.0.0`       | 1.0.1, 1.0.2   |",
		},
		{
			name:                "Non-applicable vulnerability",
			formattedDeps:       "`example-package 2.0.0`",
			maxCveScore:         "6.2",
			applicable:          "",
			fixedVersions:       []string{"2.0.1"},
			expectedDescription: "| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| 6.2      | `example-package 2.0.0`       | 2.0.1   |",
		},
		{
			name:                "No fixed versions",
			formattedDeps:       "`example-package 3.0.0`",
			maxCveScore:         "3.0",
			applicable:          "",
			fixedVersions:       []string{},
			expectedDescription: "| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| 3.0      | `example-package 3.0.0`       | No fix available   |",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := getSarifTableDescription(tc.formattedDeps, tc.maxCveScore, tc.applicable, tc.fixedVersions)
			assert.Equal(t, tc.expectedDescription, output)
		})
	}
}

func TestFindMaxCVEScore(t *testing.T) {
	testCases := []struct {
		name           string
		cves           []formats.CveRow
		expectedOutput string
		expectedError  bool
	}{
		{
			name: "CVEScore with valid float values",
			cves: []formats.CveRow{
				{Id: "CVE-2021-1234", CvssV3: "7.5"},
				{Id: "CVE-2021-5678", CvssV3: "9.2"},
			},
			expectedOutput: "9.2",
		},
		{
			name: "CVEScore with invalid float value",
			cves: []formats.CveRow{
				{Id: "CVE-2022-4321", CvssV3: "invalid"},
			},
			expectedOutput: "",
			expectedError:  true,
		},
		{
			name:           "CVEScore without values",
			cves:           []formats.CveRow{},
			expectedOutput: "0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := findMaxCVEScore(tc.cves)
			assert.False(t, tc.expectedError && err == nil)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}

func TestGetXrayIssueLocationIfValidExists(t *testing.T) {
	testDir, cleanup := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanup()
	invocation := sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation(testDir))
	file, err := os.Create(filepath.Join(testDir, "go.mod"))
	assert.NoError(t, err)
	assert.NotNil(t, file)
	defer func() { assert.NoError(t, file.Close()) }()
	file2, err := os.Create(filepath.Join(testDir, "build.gradle.kts"))
	assert.NoError(t, err)
	assert.NotNil(t, file2)
	defer func() { assert.NoError(t, file2.Close()) }()

	testCases := []struct {
		name           string
		tech           coreutils.Technology
		run            *sarif.Run
		expectedOutput *sarif.Location
	}{
		{
			name:           "No descriptor information",
			tech:           coreutils.Pip,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://Package-Descriptor"))),
		},
		{
			name:           "One descriptor information",
			tech:           coreutils.Go,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://" + filepath.Join(testDir, "go.mod")))),
		},
		{
			name:           "One descriptor information - no invocation",
			tech:           coreutils.Go,
			run:            CreateRunWithDummyResults(),
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://go.mod"))),
		},
		{
			name:           "Multiple descriptor information",
			tech:           coreutils.Gradle,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://" + filepath.Join(testDir, "build.gradle.kts")))),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := getXrayIssueLocationIfValidExists(tc.tech, tc.run)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.expectedOutput, output)
			}
		})
	}
}

func TestConvertXrayScanToSimpleJson(t *testing.T) {
	vulnerabilities := []services.Vulnerability{
		{
			IssueId:    "XRAY-1",
			Summary:    "summary-1",
			Severity:   "high",
			Components: map[string]services.Component{"component-A": {}, "component-B": {}},
		},
		{
			IssueId:    "XRAY-2",
			Summary:    "summary-2",
			Severity:   "low",
			Components: map[string]services.Component{"component-B": {}},
		},
	}
	expectedVulnerabilities := []formats.VulnerabilityOrViolationRow{
		{
			Summary: "summary-1",
			IssueId: "XRAY-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "high"},
				ImpactedDependencyName: "component-A",
			},
		},
		{
			Summary: "summary-1",
			IssueId: "XRAY-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "high"},
				ImpactedDependencyName: "component-B",
			},
		},
		{
			Summary: "summary-2",
			IssueId: "XRAY-2",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "low"},
				ImpactedDependencyName: "component-B",
			},
		},
	}

	violations := []services.Violation{
		{
			IssueId:       "XRAY-1",
			Summary:       "summary-1",
			Severity:      "high",
			WatchName:     "watch-1",
			ViolationType: "security",
			Components:    map[string]services.Component{"component-A": {}, "component-B": {}},
		},
		{
			IssueId:       "XRAY-2",
			Summary:       "summary-2",
			Severity:      "low",
			WatchName:     "watch-1",
			ViolationType: "license",
			LicenseKey:    "license-1",
			Components:    map[string]services.Component{"component-B": {}},
		},
	}
	expectedSecViolations := []formats.VulnerabilityOrViolationRow{
		{
			Summary: "summary-1",
			IssueId: "XRAY-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "high"},
				ImpactedDependencyName: "component-A",
			},
		},
		{
			Summary: "summary-1",
			IssueId: "XRAY-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "high"},
				ImpactedDependencyName: "component-B",
			},
		},
	}
	expectedLicViolations := []formats.LicenseRow{
		{
			LicenseKey: "license-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
				SeverityDetails:        formats.SeverityDetails{Severity: "low"},
				ImpactedDependencyName: "component-B",
			},
		},
	}

	licenses := []services.License{
		{
			Key:        "license-1",
			Name:       "license-1-name",
			Components: map[string]services.Component{"component-A": {}, "component-B": {}},
		},
		{
			Key:        "license-2",
			Name:       "license-2-name",
			Components: map[string]services.Component{"component-B": {}},
		},
	}
	expectedLicenses := []formats.LicenseRow{
		{
			LicenseKey:                "license-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{ImpactedDependencyName: "component-A"},
		},
		{
			LicenseKey:                "license-1",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{ImpactedDependencyName: "component-B"},
		},
		{
			LicenseKey:                "license-2",
			ImpactedDependencyDetails: formats.ImpactedDependencyDetails{ImpactedDependencyName: "component-B"},
		},
	}

	testCases := []struct {
		name            string
		result          services.ScanResponse
		includeLicenses bool
		allowedLicenses []string
		expectedOutput  formats.SimpleJsonResults
	}{
		{
			name:            "Vulnerabilities only",
			includeLicenses: false,
			allowedLicenses: nil,
			result:          services.ScanResponse{Vulnerabilities: vulnerabilities, Licenses: licenses},
			expectedOutput:  formats.SimpleJsonResults{Vulnerabilities: expectedVulnerabilities},
		},
		{
			name:            "Vulnerabilities with licenses",
			includeLicenses: true,
			allowedLicenses: nil,
			result:          services.ScanResponse{Vulnerabilities: vulnerabilities, Licenses: licenses},
			expectedOutput:  formats.SimpleJsonResults{Vulnerabilities: expectedVulnerabilities, Licenses: expectedLicenses},
		},
		{
			name:            "Vulnerabilities only - with allowed licenses",
			includeLicenses: false,
			allowedLicenses: []string{"license-1"},
			result:          services.ScanResponse{Vulnerabilities: vulnerabilities, Licenses: licenses},
			expectedOutput: formats.SimpleJsonResults{
				Vulnerabilities: expectedVulnerabilities,
				LicensesViolations: []formats.LicenseRow{
					{
						LicenseKey:                "license-2",
						ImpactedDependencyDetails: formats.ImpactedDependencyDetails{ImpactedDependencyName: "component-B"},
					},
				},
			},
		},
		{
			name:            "Violations only",
			includeLicenses: false,
			allowedLicenses: nil,
			result:          services.ScanResponse{Violations: violations, Licenses: licenses},
			expectedOutput:  formats.SimpleJsonResults{SecurityViolations: expectedSecViolations, LicensesViolations: expectedLicViolations},
		},
		{
			name:            "Violations - override allowed licenses",
			includeLicenses: false,
			allowedLicenses: []string{"license-1"},
			result:          services.ScanResponse{Violations: violations, Licenses: licenses},
			expectedOutput:  formats.SimpleJsonResults{SecurityViolations: expectedSecViolations, LicensesViolations: expectedLicViolations},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := NewAuditResults()
			results.ScaResults = append(results.ScaResults, ScaScanResult{XrayResults: []services.ScanResponse{tc.result}})
			output, err := ConvertXrayScanToSimpleJson(results, false, tc.includeLicenses, true, tc.allowedLicenses)
			if assert.NoError(t, err) {
				assert.ElementsMatch(t, tc.expectedOutput.Vulnerabilities, output.Vulnerabilities)
				assert.ElementsMatch(t, tc.expectedOutput.SecurityViolations, output.SecurityViolations)
				assert.ElementsMatch(t, tc.expectedOutput.LicensesViolations, output.LicensesViolations)
				assert.ElementsMatch(t, tc.expectedOutput.Licenses, output.Licenses)
				assert.ElementsMatch(t, tc.expectedOutput.OperationalRiskViolations, output.OperationalRiskViolations)
			}
		})
	}
}
