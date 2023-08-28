package utils

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"path/filepath"
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

func TestScanTypeErrorMsg(t *testing.T) {
	tests := []struct {
		scanner ScanType
		err     error
		wantMsg string
	}{
		{
			scanner: Applicability,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, Applicability, "an error occurred"),
		},
		{
			scanner: Applicability,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: Secrets,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: Secrets,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, Secrets, "an error occurred"),
		},
		{
			scanner: IaC,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: IaC,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, IaC, "an error occurred"),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Scanner: %s", test.scanner), func(t *testing.T) {
			gotMsg := test.scanner.FormattedError(test.err)
			if gotMsg == nil {
				assert.Nil(t, test.err)
				return
			}
			assert.Equal(t, test.wantMsg, gotMsg.Error())
		})
	}
}

func TestGetFullPathsWorkingDirs(t *testing.T) {
	currentDir, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	dir1, err := filepath.Abs("dir1")
	assert.NoError(t, err)
	dir2, err := filepath.Abs("dir2")
	assert.NoError(t, err)
	tests := []struct {
		name         string
		workingDirs  []string
		expectedDirs []string
	}{
		{
			name:         "EmptyWorkingDirs",
			workingDirs:  []string{},
			expectedDirs: []string{currentDir},
		},
		{
			name:         "ValidWorkingDirs",
			workingDirs:  []string{"dir1", "dir2"},
			expectedDirs: []string{dir1, dir2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualDirs, err := GetFullPathsWorkingDirs(test.workingDirs)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedDirs, actualDirs, "Incorrect full paths of working directories")
		})
	}
}
