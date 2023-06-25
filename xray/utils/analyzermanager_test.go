package utils

import (
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRemoveDuplicateValues(t *testing.T) {
	tests := []struct {
		testedSlice    []string
		expectedResult []string
	}{
		{testedSlice: []string{"1", "1", "1", "3"}, expectedResult: []string{"1", "3"}},
		{testedSlice: []string{}, expectedResult: []string{}},
		{testedSlice: []string{"1", "2", "3", "4"}, expectedResult: []string{"1", "2", "3", "4"}},
		{testedSlice: []string{"1", "6", "1", "6", "2"}, expectedResult: []string{"1", "6", "2"}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, RemoveDuplicateValues(test.testedSlice))
	}
}

func TestGetResultFileName(t *testing.T) {
	fileNameValue := "fileNameValue"
	tests := []struct {
		result         *sarif.Result
		expectedOutput string
	}{
		{result: &sarif.Result{
			Locations: []*sarif.Location{
				{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: nil}}},
			}},
			expectedOutput: ""},
		{result: &sarif.Result{
			Locations: []*sarif.Location{
				{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &fileNameValue}}},
			}},
			expectedOutput: fileNameValue},
		{result: &sarif.Result{},
			expectedOutput: ""},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetResultFileName(test.result))
	}

}

func TestGetResultLocationInFile(t *testing.T) {
	startLine := 19
	startColumn := 25

	tests := []struct {
		result         *sarif.Result
		expectedOutput string
	}{
		{result: &sarif.Result{Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{
				StartLine:   &startLine,
				StartColumn: &startColumn,
			}}}}},
			expectedOutput: "19:25"},
		{result: &sarif.Result{Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{
				StartLine:   nil,
				StartColumn: &startColumn,
			}}}}},
			expectedOutput: ""},
		{result: &sarif.Result{Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{
				StartLine:   &startLine,
				StartColumn: nil,
			}}}}},
			expectedOutput: ""},
		{result: &sarif.Result{Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{
				StartLine:   nil,
				StartColumn: nil,
			}}}}},
			expectedOutput: ""},
		{result: &sarif.Result{},
			expectedOutput: ""},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetResultLocationInFile(test.result))
	}
}

func TestExtractRelativePath(t *testing.T) {
	tests := []struct {
		secretPath     string
		projectPath    string
		expectedResult string
	}{
		{secretPath: "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: "/tests/req.nodejs/file.js"},
		{secretPath: "invalidSecretPath",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: "invalidSecretPath"},
		{secretPath: "",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: ""},
		{secretPath: "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "invalidProjectPath", expectedResult: "/Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, ExtractRelativePath(test.secretPath, test.projectPath))
	}
}

func TestGetResultSeverity(t *testing.T) {
	levelValueHigh := "error"
	levelValueMedium := "warning"
	levelValueLow := "info"

	tests := []struct {
		result           *sarif.Result
		expectedSeverity string
	}{
		{result: &sarif.Result{},
			expectedSeverity: "Medium"},
		{result: &sarif.Result{Level: &levelValueHigh},
			expectedSeverity: "High"},
		{result: &sarif.Result{Level: &levelValueMedium},
			expectedSeverity: "Medium"},
		{result: &sarif.Result{Level: &levelValueLow},
			expectedSeverity: "Low"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedSeverity, GetResultSeverity(test.result))
	}
}

func TestExcludeScan(t *testing.T) {
	tests := []struct {
		excludedScanList []string
		scanToCheck      string
		expectedResult   bool
	}{
		{excludedScanList: []string{"secrets", "iac", "contextual_analysis"}, scanToCheck: "contextual_analysis", expectedResult: true},
		{excludedScanList: []string{"secrets", "iac"}, scanToCheck: "contextual_analysis", expectedResult: false},
		{excludedScanList: []string{}, scanToCheck: "contextual_analysis", expectedResult: false},
		{excludedScanList: []string{"", "unsupported_Scan_type", "contextual_analysis"}, scanToCheck: "contextual_analysis", expectedResult: true},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, ExcludeScan(test.excludedScanList, test.scanToCheck))
	}
}
