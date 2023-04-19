package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"testing"

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
		err := PrintViolationsTable(test.violations, &jas.ExtendedScanResults{}, false, true, true)
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
		{"docker://jfrog/artifactory-oss:latest", "jfrog/artifactory-oss", "latest", "Docker"},
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
	cves := []services.Cve{}
	expectedKey := "test-dependency:1.0::true"
	key := getUniqueKey(vulnerableDependency, vulnerableVersion, cves, true)
	assert.Equal(t, expectedKey, key)

	cves = []services.Cve{
		{Id: "CVE-1"},
		{Id: "CVE-2"},
	}
	expectedKey = "test-dependency:1.0:CVE-1:false"
	key = getUniqueKey(vulnerableDependency, vulnerableVersion, cves, false)
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
