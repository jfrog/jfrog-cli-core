package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/owenrumney/go-sarif/v2/sarif"

	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
)

// The test only checks cases of returning an error in case of a violation with FailBuild == true
func TestPrintViolationsTable(t *testing.T) {
	components := map[string]services.Component{"gav://antparent:ant:1.6.5": {}}
	tests := []struct {
		violations    []services.Violation
		expectedError bool
	}{
		{[]services.Violation{{Components: components, FailBuild: false}, {Components: components, FailBuild: false}, {Components: components, FailBuild: false}}, false},
		{[]services.Violation{{Components: components, FailBuild: false}, {Components: components, FailBuild: true}, {Components: components, FailBuild: false}}, true},
		{[]services.Violation{{Components: components, FailBuild: true}, {Components: components, FailBuild: true}, {Components: components, FailBuild: true}}, true},
	}

	for _, test := range tests {
		err := PrintViolationsTable(test.violations, &ExtendedScanResults{}, false, true, services.Binary)
		assert.NoError(t, err)
		if CheckIfFailBuild([]services.ScanResponse{{Violations: test.violations}}) {
			err = NewFailBuildError()
		}
		assert.Equal(t, test.expectedError, err != nil)
	}
}

func TestSplitComponentId(t *testing.T) {
	tests := []struct {
		componentId         string
		expectedCompName    string
		expectedCompVersion string
		expectedCompType    string
	}{
		{"gav://antparent:ant:1.6.5", "antparent:ant", "1.6.5", "Maven"},
		{"docker://jfrog/artifactory-oss:latest", "jfrog/artifactory-oss", "latest", "DockerScan"},
		{"rpm://7:rpm-python:7:4.11.3-43.el7", "rpm-python", "7:4.11.3-43.el7", "RPM"},
		{"rpm://rpm-python:7:4.11.3-43.el7", "rpm-python", "7:4.11.3-43.el7", "RPM"},
		{"deb://ubuntu:trustee:acl:2.2.49-2", "ubuntu:trustee:acl", "2.2.49-2", "Debian"},
		{"nuget://log4net:9.0.1", "log4net", "9.0.1", "NuGet"},
		{"generic://sha256:244fd47e07d1004f0aed9c156aa09083c82bf8944eceb67c946ff7430510a77b/foo.jar", "foo.jar", "", "Generic"},
		{"npm://mocha:2.4.5", "mocha", "2.4.5", "npm"},
		{"pip://raven:5.13.0", "raven", "5.13.0", "Python"},
		{"composer://nunomaduro/collision:1.1", "nunomaduro/collision", "1.1", "Composer"},
		{"go://github.com/ethereum/go-ethereum:1.8.2", "github.com/ethereum/go-ethereum", "1.8.2", "Go"},
		{"alpine://3.7:htop:2.0.2-r0", "3.7:htop", "2.0.2-r0", "Alpine"},
		{"invalid-component-id:1.0.0", "invalid-component-id:1.0.0", "", ""},
	}

	for _, test := range tests {
		actualCompName, actualCompVersion, actualCompType := SplitComponentId(test.componentId)
		assert.Equal(t, test.expectedCompName, actualCompName)
		assert.Equal(t, test.expectedCompVersion, actualCompVersion)
		assert.Equal(t, test.expectedCompType, actualCompType)
	}
}

func TestGetDirectComponents(t *testing.T) {
	tests := []struct {
		impactPaths             [][]services.ImpactPathNode
		expectedComponentRows   []formats.ComponentRow
		expectedConvImpactPaths [][]formats.ComponentRow
	}{
		{[][]services.ImpactPathNode{{services.ImpactPathNode{ComponentId: "gav://jfrog:pack:1.2.3"}}}, []formats.ComponentRow{{Name: "jfrog:pack", Version: "1.2.3"}}, [][]formats.ComponentRow{{{Name: "jfrog:pack", Version: "1.2.3"}}}},
		{[][]services.ImpactPathNode{{services.ImpactPathNode{ComponentId: "gav://jfrog:pack1:1.2.3"}, services.ImpactPathNode{ComponentId: "gav://jfrog:pack2:1.2.3"}}}, []formats.ComponentRow{{Name: "jfrog:pack2", Version: "1.2.3"}}, [][]formats.ComponentRow{{{Name: "jfrog:pack1", Version: "1.2.3"}, {Name: "jfrog:pack2", Version: "1.2.3"}}}},
		{[][]services.ImpactPathNode{{services.ImpactPathNode{ComponentId: "gav://jfrog:pack1:1.2.3"}, services.ImpactPathNode{ComponentId: "gav://jfrog:pack21:1.2.3"}, services.ImpactPathNode{ComponentId: "gav://jfrog:pack3:1.2.3"}}, {services.ImpactPathNode{ComponentId: "gav://jfrog:pack1:1.2.3"}, services.ImpactPathNode{ComponentId: "gav://jfrog:pack22:1.2.3"}, services.ImpactPathNode{ComponentId: "gav://jfrog:pack3:1.2.3"}}}, []formats.ComponentRow{{Name: "jfrog:pack21", Version: "1.2.3"}, {Name: "jfrog:pack22", Version: "1.2.3"}}, [][]formats.ComponentRow{{{Name: "jfrog:pack1", Version: "1.2.3"}, {Name: "jfrog:pack21", Version: "1.2.3"}, {Name: "jfrog:pack3", Version: "1.2.3"}}, {{Name: "jfrog:pack1", Version: "1.2.3"}, {Name: "jfrog:pack22", Version: "1.2.3"}, {Name: "jfrog:pack3", Version: "1.2.3"}}}},
	}

	for _, test := range tests {
		actualComponentRows, actualConvImpactPaths := getDirectComponentsAndImpactPaths(test.impactPaths)
		assert.ElementsMatch(t, test.expectedComponentRows, actualComponentRows)
		assert.ElementsMatch(t, test.expectedConvImpactPaths, actualConvImpactPaths)
	}
}

func TestGetOperationalRiskReadableData(t *testing.T) {
	tests := []struct {
		violation       services.Violation
		expectedResults *operationalRiskViolationReadableData
	}{
		{
			services.Violation{IsEol: nil, LatestVersion: "", NewerVersions: nil,
				Cadence: nil, Commits: nil, Committers: nil, RiskReason: "", EolMessage: ""},
			&operationalRiskViolationReadableData{"N/A", "N/A", "N/A", "N/A", "", "", "N/A", "N/A"},
		},
		{
			services.Violation{IsEol: newBoolPtr(true), LatestVersion: "1.2.3", NewerVersions: newIntPtr(5),
				Cadence: newFloat64Ptr(3.5), Commits: newInt64Ptr(55), Committers: newIntPtr(10), EolMessage: "no maintainers", RiskReason: "EOL"},
			&operationalRiskViolationReadableData{"true", "3.5", "55", "10", "no maintainers", "EOL", "1.2.3", "5"},
		},
	}

	for _, test := range tests {
		results := getOperationalRiskViolationReadableData(test.violation)
		assert.Equal(t, test.expectedResults, results)
	}
}

func TestIsImpactPathIsSubset(t *testing.T) {
	testCases := []struct {
		name                           string
		target, source, expectedResult []services.ImpactPathNode
	}{
		{"subset found in both target and source",
			[]services.ImpactPathNode{{ComponentId: "B"}, {ComponentId: "C"}},
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "C"}},
			[]services.ImpactPathNode{{ComponentId: "B"}, {ComponentId: "C"}},
		},
		{"subset not found in both target and source",
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "D"}},
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "C"}},
			[]services.ImpactPathNode{},
		},
		{"target and source are identical",
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}},
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}},
			[]services.ImpactPathNode{{ComponentId: "A"}, {ComponentId: "B"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isImpactPathIsSubset(tc.target, tc.source)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestAppendUniqueFixVersions(t *testing.T) {
	testCases := []struct {
		targetFixVersions []string
		sourceFixVersions []string
		expectedResult    []string
	}{
		{
			targetFixVersions: []string{"1.0", "1.1"},
			sourceFixVersions: []string{"2.0", "2.1"},
			expectedResult:    []string{"1.0", "1.1", "2.0", "2.1"},
		},
		{
			targetFixVersions: []string{"1.0", "1.1"},
			sourceFixVersions: []string{"1.1", "2.0"},
			expectedResult:    []string{"1.0", "1.1", "2.0"},
		},
		{
			targetFixVersions: []string{},
			sourceFixVersions: []string{"1.0", "1.1"},
			expectedResult:    []string{"1.0", "1.1"},
		},
		{
			targetFixVersions: []string{"1.0", "1.1"},
			sourceFixVersions: []string{},
			expectedResult:    []string{"1.0", "1.1"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("target:%v, source:%v", tc.targetFixVersions, tc.sourceFixVersions), func(t *testing.T) {
			result := appendUniqueFixVersions(tc.targetFixVersions, tc.sourceFixVersions...)
			assert.ElementsMatch(t, tc.expectedResult, result)
		})
	}
}

func TestGetUniqueKey(t *testing.T) {
	vulnerableDependency := "test-dependency"
	vulnerableVersion := "1.0"
	expectedKey := "test-dependency:1.0:XRAY-12234:true"
	key := GetUniqueKey(vulnerableDependency, vulnerableVersion, "XRAY-12234", true)
	assert.Equal(t, expectedKey, key)

	expectedKey = "test-dependency:1.0:XRAY-12143:false"
	key = GetUniqueKey(vulnerableDependency, vulnerableVersion, "XRAY-12143", false)
	assert.Equal(t, expectedKey, key)
}

func TestAppendUniqueImpactPathsForMultipleRoots(t *testing.T) {
	testCases := []struct {
		name           string
		target         [][]services.ImpactPathNode
		source         [][]services.ImpactPathNode
		expectedResult [][]services.ImpactPathNode
	}{
		{
			name: "subset is found in both target and source",
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "D"}, {ComponentId: "E"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "D"}, {ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
		},
		{
			name: "subset is not found in both target and source",
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "D"}, {ComponentId: "E"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "B"}, {ComponentId: "C"}},
				{{ComponentId: "D"}, {ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
		},
		{
			name:   "target slice is empty",
			target: [][]services.ImpactPathNode{},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
		},
		{
			name: "source slice is empty",
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			source: [][]services.ImpactPathNode{},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
		},
		{
			name: "target and source slices are identical",
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
		},
		{
			name: "target and source slices contain multiple subsets",
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}, {ComponentId: "E"}},
				{{ComponentId: "C"}, {ComponentId: "D"}, {ComponentId: "F"}},
				{{ComponentId: "G"}, {ComponentId: "H"}},
			},
			expectedResult: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
				{{ComponentId: "G"}, {ComponentId: "H"}},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedResult, appendUniqueImpactPathsForMultipleRoots(test.target, test.source))
		})
	}
}

func TestGetImpactPathKey(t *testing.T) {
	testCases := []struct {
		path        []services.ImpactPathNode
		expectedKey string
	}{
		{
			path: []services.ImpactPathNode{
				{ComponentId: "A"},
				{ComponentId: "B"},
			},
			expectedKey: "B",
		},
		{
			path: []services.ImpactPathNode{
				{ComponentId: "A"},
			},
			expectedKey: "A",
		},
	}

	for _, test := range testCases {
		key := getImpactPathKey(test.path)
		assert.Equal(t, test.expectedKey, key)
	}
}

func TestAppendUniqueImpactPaths(t *testing.T) {
	testCases := []struct {
		name          string
		multipleRoots bool
		target        [][]services.ImpactPathNode
		source        [][]services.ImpactPathNode
		expected      [][]services.ImpactPathNode
	}{
		{
			name:          "Test case 1: Unique impact paths found",
			multipleRoots: false,
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}},
				{{ComponentId: "B"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "C"}},
				{{ComponentId: "D"}},
			},
			expected: [][]services.ImpactPathNode{
				{{ComponentId: "A"}},
				{{ComponentId: "B"}},
				{{ComponentId: "C"}},
				{{ComponentId: "D"}},
			},
		},
		{
			name:          "Test case 2: No unique impact paths found",
			multipleRoots: false,
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}},
				{{ComponentId: "B"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "A"}},
				{{ComponentId: "B"}},
			},
			expected: [][]services.ImpactPathNode{
				{{ComponentId: "A"}},
				{{ComponentId: "B"}},
			},
		},
		{
			name:          "Test case 3: paths in source are not in target",
			multipleRoots: false,
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
			expected: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
				{{ComponentId: "E"}},
				{{ComponentId: "F"}, {ComponentId: "G"}},
			},
		},
		{
			name:          "Test case 4: paths in source are already in target",
			multipleRoots: false,
			target: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			source: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
			expected: [][]services.ImpactPathNode{
				{{ComponentId: "A"}, {ComponentId: "B"}},
				{{ComponentId: "C"}, {ComponentId: "D"}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := appendUniqueImpactPaths(tc.target, tc.source, tc.multipleRoots)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetSeveritiesFormat(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
		expectedError  error
	}{
		// Test supported severity
		{input: "critical", expectedOutput: "Critical", expectedError: nil},
		{input: "hiGH", expectedOutput: "High", expectedError: nil},
		{input: "Low", expectedOutput: "Low", expectedError: nil},
		{input: "MedIum", expectedOutput: "Medium", expectedError: nil},
		{input: "", expectedOutput: "", expectedError: nil},
		// Test unsupported severity
		{input: "invalid_severity", expectedOutput: "", expectedError: errors.New("only the following severities are supported")},
	}

	for _, tc := range testCases {
		output, err := GetSeveritiesFormat(tc.input)
		if err != nil {
			assert.Contains(t, err.Error(), tc.expectedError.Error())
		} else {
			assert.Equal(t, tc.expectedError, err)
		}
		assert.Equal(t, tc.expectedOutput, output)
	}
}

func TestGetApplicableCveValue(t *testing.T) {
	testCases := []struct {
		scanResults    *ExtendedScanResults
		cves           []services.Cve
		expectedResult ApplicabilityStatus
		expectedCves   []formats.CveRow
	}{
		{
			scanResults:    &ExtendedScanResults{EntitledForJas: false},
			expectedResult: NotScanned,
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(
						CreateResultWithOneLocation("fileName1", 0, 1, 0, 0, "snippet1", "applic_testCve1", "info"),
						CreateDummyPassingResult("applic_testCve2"),
					),
				},
				EntitledForJas: true,
			},
			cves:           nil,
			expectedResult: ApplicabilityUndetermined,
			expectedCves:   nil,
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(
						CreateDummyPassingResult("applic_testCve1"),
						CreateResultWithOneLocation("fileName2", 1, 0, 0, 0, "snippet2", "applic_testCve2", "warning"),
					),
				},
				EntitledForJas: true,
			},
			cves:           []services.Cve{{Id: "testCve2"}},
			expectedResult: Applicable,
			expectedCves:   []formats.CveRow{{Id: "testCve2", Applicability: &formats.Applicability{Status: string(Applicable)}}},
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(
						CreateDummyPassingResult("applic_testCve1"),
						CreateResultWithOneLocation("fileName3", 0, 1, 0, 0, "snippet3", "applic_testCve2", "info"),
					),
				},
				EntitledForJas: true,
			},
			cves:           []services.Cve{{Id: "testCve3"}},
			expectedResult: ApplicabilityUndetermined,
			expectedCves:   []formats.CveRow{{Id: "testCve3"}},
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(
						CreateDummyPassingResult("applic_testCve1"),
						CreateDummyPassingResult("applic_testCve2"),
					),
				},
				EntitledForJas: true,
			},
			cves:           []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}},
			expectedResult: NotApplicable,
			expectedCves:   []formats.CveRow{{Id: "testCve1", Applicability: &formats.Applicability{Status: string(NotApplicable)}}, {Id: "testCve2", Applicability: &formats.Applicability{Status: string(NotApplicable)}}},
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(
						CreateDummyPassingResult("applic_testCve1"),
						CreateResultWithOneLocation("fileName4", 1, 0, 0, 0, "snippet", "applic_testCve2", "warning"),
					),
				},
				EntitledForJas: true,
			},
			cves:           []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}},
			expectedResult: Applicable,
			expectedCves:   []formats.CveRow{{Id: "testCve1", Applicability: &formats.Applicability{Status: string(NotApplicable)}}, {Id: "testCve2", Applicability: &formats.Applicability{Status: string(Applicable)}}},
		},
		{
			scanResults: &ExtendedScanResults{
				ApplicabilityScanResults: []*sarif.Run{
					CreateRunWithDummyResults(CreateDummyPassingResult("applic_testCve1")),
				},
				EntitledForJas: true},
			cves:           []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}},
			expectedResult: ApplicabilityUndetermined,
			expectedCves:   []formats.CveRow{{Id: "testCve1", Applicability: &formats.Applicability{Status: string(NotApplicable)}}, {Id: "testCve2"}},
		},
	}

	for _, testCase := range testCases {
		cves := convertCves(testCase.cves)
		for i := range cves {
			cves[i].Applicability = getCveApplicabilityField(cves[i], testCase.scanResults.ApplicabilityScanResults, nil)
		}
		applicableValue := getApplicableCveStatus(testCase.scanResults.EntitledForJas, testCase.scanResults.ApplicabilityScanResults, cves)
		assert.Equal(t, testCase.expectedResult, applicableValue)
		if assert.True(t, len(testCase.expectedCves) == len(cves)) {
			for i := range cves {
				if testCase.expectedCves[i].Applicability != nil && assert.NotNil(t, cves[i].Applicability) {
					assert.Equal(t, testCase.expectedCves[i].Applicability.Status, cves[i].Applicability.Status)
				}
			}
		}
	}
}

func TestSortVulnerabilityOrViolationRows(t *testing.T) {
	testCases := []struct {
		name          string
		rows          []formats.VulnerabilityOrViolationRow
		expectedOrder []string
	}{
		{
			name: "Sort by severity with different severity values",
			rows: []formats.VulnerabilityOrViolationRow{
				{
					Summary: "Summary 1",
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "High",
							SeverityNumValue: 9,
						},
						ImpactedDependencyName:    "Dependency 1",
						ImpactedDependencyVersion: "1.0.0",
					},
					FixedVersions: []string{},
				},
				{
					Summary: "Summary 2",
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 12,
						},
						ImpactedDependencyName:    "Dependency 2",
						ImpactedDependencyVersion: "2.0.0",
					},
					FixedVersions: []string{"1.0.0"},
				},
				{
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Medium",
							SeverityNumValue: 6,
						},
						ImpactedDependencyName:    "Dependency 3",
						ImpactedDependencyVersion: "3.0.0",
					},
					Summary:       "Summary 3",
					FixedVersions: []string{},
				},
			},
			expectedOrder: []string{"Dependency 2", "Dependency 1", "Dependency 3"},
		},
		{
			name: "Sort by severity with same severity values, but different fixed versions",
			rows: []formats.VulnerabilityOrViolationRow{
				{
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 12,
						},
						ImpactedDependencyName:    "Dependency 1",
						ImpactedDependencyVersion: "1.0.0",
					},
					Summary:       "Summary 1",
					FixedVersions: []string{"1.0.0"},
				},
				{
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 12,
						},
						ImpactedDependencyName:    "Dependency 2",
						ImpactedDependencyVersion: "2.0.0",
					},
					Summary:       "Summary 2",
					FixedVersions: []string{},
				},
			},
			expectedOrder: []string{"Dependency 1", "Dependency 2"},
		},
		{
			name: "Sort by severity with same severity values different applicability",
			rows: []formats.VulnerabilityOrViolationRow{
				{
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 13,
						},
						ImpactedDependencyName:    "Dependency 1",
						ImpactedDependencyVersion: "1.0.0",
					},
					Summary:       "Summary 1",
					Applicable:    Applicable.String(),
					FixedVersions: []string{"1.0.0"},
				},
				{
					Summary:    "Summary 2",
					Applicable: NotApplicable.String(),
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 11,
						},
						ImpactedDependencyName:    "Dependency 2",
						ImpactedDependencyVersion: "2.0.0",
					},
				},
				{
					Summary:    "Summary 3",
					Applicable: ApplicabilityUndetermined.String(),
					ImpactedDependencyDetails: formats.ImpactedDependencyDetails{
						SeverityDetails: formats.SeverityDetails{
							Severity:         "Critical",
							SeverityNumValue: 12,
						},
						ImpactedDependencyName:    "Dependency 3",
						ImpactedDependencyVersion: "2.0.0",
					},
				},
			},
			expectedOrder: []string{"Dependency 1", "Dependency 3", "Dependency 2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sortVulnerabilityOrViolationRows(tc.rows)

			for i, row := range tc.rows {
				assert.Equal(t, tc.expectedOrder[i], row.ImpactedDependencyName)
			}
		})
	}
}

func TestShouldDisqualifyEvidence(t *testing.T) {
	testCases := []struct {
		name       string
		component  map[string]services.Component
		filePath   string
		disqualify bool
	}{
		{
			name:       "package folders",
			component:  map[string]services.Component{"npm://protobufjs:6.11.2": {}},
			filePath:   "file:///Users/jfrog/test/node_modules/protobufjs/src/badCode.js",
			disqualify: true,
		}, {
			name:       "nested folders",
			component:  map[string]services.Component{"npm://protobufjs:6.11.2": {}},
			filePath:   "file:///Users/jfrog/test/node_modules/someDep/node_modules/protobufjs/src/badCode.js",
			disqualify: true,
		}, {
			name:       "applicability in node modules",
			component:  map[string]services.Component{"npm://protobufjs:6.11.2": {}},
			filePath:   "file:///Users/jfrog/test/node_modules/mquery/src/badCode.js",
			disqualify: false,
		}, {
			// Only npm supported
			name:       "not npm",
			component:  map[string]services.Component{"yarn://protobufjs:6.11.2": {}},
			filePath:   "file:///Users/jfrog/test/node_modules/protobufjs/src/badCode.js",
			disqualify: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.disqualify, shouldDisqualifyEvidence(tc.component, tc.filePath))
		})
	}
}

func TestPrepareIac(t *testing.T) {
	testCases := []struct {
		name           string
		input          []*sarif.Run
		expectedOutput []formats.SourceCodeRow
	}{
		{
			name:           "No Iac run",
			input:          []*sarif.Run{},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Iac run - no results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
			},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Iac run - with results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(
					CreateResultWithLocations("iac finding", "rule1", "info",
						CreateLocation("file://wd/file", 1, 2, 3, 4, "snippet"),
						CreateLocation("file://wd/file2", 5, 6, 7, 8, "other-snippet"),
					),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				}),
				CreateRunWithDummyResults(
					CreateResultWithLocations("other iac finding", "rule2", "error",
						CreateLocation("file://wd2/file3", 1, 2, 3, 4, "snippet"),
					),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd2")),
				}),
			},
			expectedOutput: []formats.SourceCodeRow{
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "High",
						SeverityNumValue: 13,
					},
					Finding: "other iac finding",
					Location: formats.Location{
						File:        "file3",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "snippet",
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "iac finding",
					Location: formats.Location{
						File:        "file",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "snippet",
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "iac finding",
					Location: formats.Location{
						File:        "file2",
						StartLine:   5,
						StartColumn: 6,
						EndLine:     7,
						EndColumn:   8,
						Snippet:     "other-snippet",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.expectedOutput, prepareIacs(tc.input, false))
		})
	}
}

func TestPrepareSecrets(t *testing.T) {
	testCases := []struct {
		name           string
		input          []*sarif.Run
		expectedOutput []formats.SourceCodeRow
	}{
		{
			name:           "No Secret run",
			input:          []*sarif.Run{},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Secret run - no results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
			},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Secret run - with results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(
					CreateResultWithLocations("secret finding", "rule1", "info",
						CreateLocation("file://wd/file", 1, 2, 3, 4, "some-secret-snippet"),
						CreateLocation("file://wd/file2", 5, 6, 7, 8, "other-secret-snippet"),
					),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				}),
				CreateRunWithDummyResults(
					CreateResultWithLocations("other secret finding", "rule2", "note",
						CreateLocation("file://wd2/file3", 1, 2, 3, 4, "some-secret-snippet"),
					),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd2")),
				}),
			},
			expectedOutput: []formats.SourceCodeRow{
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Low",
						SeverityNumValue: 9,
					},
					Finding: "other secret finding",
					Location: formats.Location{
						File:        "file3",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "some-secret-snippet",
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "secret finding",
					Location: formats.Location{
						File:        "file",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "some-secret-snippet",
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "secret finding",
					Location: formats.Location{
						File:        "file2",
						StartLine:   5,
						StartColumn: 6,
						EndLine:     7,
						EndColumn:   8,
						Snippet:     "other-secret-snippet",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.expectedOutput, prepareSecrets(tc.input, false))
		})
	}
}

func TestPrepareSast(t *testing.T) {
	testCases := []struct {
		name           string
		input          []*sarif.Run
		expectedOutput []formats.SourceCodeRow
	}{
		{
			name:           "No Sast run",
			input:          []*sarif.Run{},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Sast run - no results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(),
			},
			expectedOutput: []formats.SourceCodeRow{},
		},
		{
			name: "Prepare Sast run - with results",
			input: []*sarif.Run{
				CreateRunWithDummyResults(),
				CreateRunWithDummyResults(
					CreateResultWithLocations("sast finding", "rule1", "info",
						CreateLocation("file://wd/file", 1, 2, 3, 4, "snippet"),
						CreateLocation("file://wd/file2", 5, 6, 7, 8, "other-snippet"),
					).WithCodeFlows([]*sarif.CodeFlow{
						CreateCodeFlow(CreateThreadFlow(
							CreateLocation("file://wd/file2", 0, 2, 0, 2, "snippetA"),
							CreateLocation("file://wd/file", 1, 2, 3, 4, "snippet"),
						)),
						CreateCodeFlow(CreateThreadFlow(
							CreateLocation("file://wd/file4", 1, 0, 1, 8, "snippetB"),
							CreateLocation("file://wd/file", 1, 2, 3, 4, "snippet"),
						)),
					}),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				}),
				CreateRunWithDummyResults(
					CreateResultWithLocations("other sast finding", "rule2", "error",
						CreateLocation("file://wd2/file3", 1, 2, 3, 4, "snippet"),
					),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd2")),
				}),
			},
			expectedOutput: []formats.SourceCodeRow{
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "High",
						SeverityNumValue: 13,
					},
					Finding: "other sast finding",
					Location: formats.Location{
						File:        "file3",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "snippet",
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "sast finding",
					Location: formats.Location{
						File:        "file",
						StartLine:   1,
						StartColumn: 2,
						EndLine:     3,
						EndColumn:   4,
						Snippet:     "snippet",
					},
					CodeFlow: [][]formats.Location{
						{
							{
								File:        "file2",
								StartLine:   0,
								StartColumn: 2,
								EndLine:     0,
								EndColumn:   2,
								Snippet:     "snippetA",
							},
							{
								File:        "file",
								StartLine:   1,
								StartColumn: 2,
								EndLine:     3,
								EndColumn:   4,
								Snippet:     "snippet",
							},
						},
						{
							{
								File:        "file4",
								StartLine:   1,
								StartColumn: 0,
								EndLine:     1,
								EndColumn:   8,
								Snippet:     "snippetB",
							},
							{
								File:        "file",
								StartLine:   1,
								StartColumn: 2,
								EndLine:     3,
								EndColumn:   4,
								Snippet:     "snippet",
							},
						},
					},
				},
				{
					SeverityDetails: formats.SeverityDetails{
						Severity:         "Medium",
						SeverityNumValue: 11,
					},
					Finding: "sast finding",
					Location: formats.Location{
						File:        "file2",
						StartLine:   5,
						StartColumn: 6,
						EndLine:     7,
						EndColumn:   8,
						Snippet:     "other-snippet",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.expectedOutput, prepareSast(tc.input, false))
		})
	}
}

func newBoolPtr(v bool) *bool {
	return &v
}

func newIntPtr(v int) *int {
	return &v
}

func newInt64Ptr(v int64) *int64 {
	return &v
}

func newFloat64Ptr(v float64) *float64 {
	return &v
}
