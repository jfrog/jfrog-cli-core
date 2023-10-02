package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
)

func TestGetVulnerabilityOrViolationSarifHeadline(t *testing.T) {
	assert.Equal(t, "[CVE-2022-1234] loadsh 1.4.1", getVulnerabilityOrViolationSarifHeadline("loadsh", "1.4.1", "CVE-2022-1234"))
	assert.NotEqual(t, "[CVE-2022-1234] loadsh 1.4.1", getVulnerabilityOrViolationSarifHeadline("loadsh", "1.2.1", "CVE-2022-1234"))
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
		markdown       bool
		expectedOutput *sarif.Location
	}{
		{
			name:           "No descriptor information",
			tech:           coreutils.Pip,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			markdown:       false,
			expectedOutput: nil,
		},
		{
			name:           "No descriptor information - markdown",
			tech:           coreutils.Poetry,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			markdown:       true,
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri(coreutils.Poetry.ToFormal() + " Package Descriptor"))),
		},
		{
			name:           "One descriptor information",
			tech:           coreutils.Go,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			markdown:       false,
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://" + filepath.Join(testDir, "go.mod")))),
		},
		{
			name:           "One descriptor information - markdown",
			tech:           coreutils.Maven,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			markdown:       true,
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://pom.xml"))),
		},
		{
			name:           "Multiple descriptor information",
			tech:           coreutils.Gradle,
			run:            CreateRunWithDummyResults().WithInvocations([]*sarif.Invocation{invocation}),
			markdown:       false,
			expectedOutput: sarif.NewLocation().WithPhysicalLocation(sarif.NewPhysicalLocation().WithArtifactLocation(sarif.NewArtifactLocation().WithUri("file://" + filepath.Join(testDir, "build.gradle.kts")))),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := getXrayIssueLocationIfValidExists(tc.tech, tc.run, tc.markdown)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.expectedOutput, output)
			}
		})
	}
}
