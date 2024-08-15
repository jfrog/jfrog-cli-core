package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockCommandSummary struct {
	CommandSummaryInterface
}

type BasicStruct struct {
	Field1 string
	Field2 int
}

func (tcs *mockCommandSummary) GenerateMarkdownFromFiles(_ []string, _ map[Index]map[string]string) (finalMarkdown string, err error) {
	return "mockMarkdown", nil
}

// Without output dir env, New should return an error.
func TestInitWithoutOutputDir(t *testing.T) {
	_, err := New(&mockCommandSummary{}, "testsCommands")
	assert.Error(t, err)
}

// Verifies the behavior of recording simple data.
func TestSimpleRecord(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name         string
		dirName      string
		originalData interface{}
	}{
		{
			name:         "Test with a simple object",
			dirName:      "testDir",
			originalData: map[string]string{"key": "value"},
		},
		{
			name:         "Test with a string",
			dirName:      "testDir3",
			originalData: "test string",
		},
		{
			name:    "Test with a basic struct",
			dirName: "testDir4",
			originalData: BasicStruct{
				Field1: "test string",
				Field2: 123,
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare a new CommandSummary for each test case
			cs, cleanUp := prepareTest(t)
			defer func() {
				cleanUp()
			}()
			// Save data to file
			err := cs.Record(tc.originalData)
			assert.NoError(t, err)

			// Verify file has been saved
			dataFiles, _, err := cs.getDataFilesPaths()
			assert.NoError(t, err)
			assert.NotEqual(t, 0, len(dataFiles))

			// Verify that data has not been corrupted
			loadedData, err := unmarshalData(tc.originalData, dataFiles[0])
			assert.NoError(t, err)
			assert.EqualValues(t, tc.originalData, loadedData)
		})
	}
}

// Tests the behavior of recording with indexes.
// Ensures a nested file structure is created for future use.
func TestIndexedRecord(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                     string
		dirName                  string
		originalData             interface{}
		summaryIndex             Index
		expectedDirectoryMapping map[Index]map[string]string
		recordArgs               []string
	}{
		// Build-Scan result file should contain the build name and build number
		{
			name:         "Record build scan result",
			summaryIndex: BuildScan,
			expectedDirectoryMapping: map[Index]map[string]string{
				BuildScan: {
					"buildName-buildNumber": "buildScanResults",
				},
			},
			recordArgs: []string{"buildName", "buildNumber"},
		},
		// Binary files should contain a full path to the file
		// To handle the case where we scan different binaries but with identical names.
		{
			name:         "Record binary scan result",
			summaryIndex: BinariesScan,
			expectedDirectoryMapping: map[Index]map[string]string{
				BinariesScan: {
					"path-to-some-binary.exe": "binaryResults",
				},
			},
			recordArgs: []string{"path/to/some-binary.exe"},
		},
		// Docker images files should be saved without any slashes or colons
		{
			name:         "Record docker scan result",
			summaryIndex: DockerScan,
			expectedDirectoryMapping: map[Index]map[string]string{
				DockerScan: {
					"linux-amd64-my-image-latest": "dockerResults",
				},
			},
			recordArgs: []string{"linux/amd64/my-image:latest"},
		},
		// There could be multiple sarif reports in the same directory
		{
			name:         "Record sarif report",
			summaryIndex: SarifReport,
			expectedDirectoryMapping: map[Index]map[string]string{
				SarifReport: {
					"*.sarif": "sarifReport",
				},
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare a new CommandSummary for each test case
			cs, cleanUp := prepareTest(t)
			defer func() {
				cleanUp()
			}()

			// Save data to nested folders
			err := cs.RecordWithIndex(tc.originalData, tc.summaryIndex, tc.recordArgs...)
			assert.NoError(t, err)

			// Verify file has been saved
			_, nestedFiles, err := cs.getDataFilesPaths()
			assert.NoError(t, err)

			// Verify nested files
			verifyCurrentMapping(t, tc.expectedDirectoryMapping, nestedFiles)
		})
	}
}

// Sarif is a special indexed case, where there could be multiple indexed files.
// Therefore, the names are random and should be saved in the same directory.
func TestSarifMultipleReports(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name         string
		originalData interface{}
		summaryIndex Index
	}{
		{
			name:         "Record sarif report",
			summaryIndex: SarifReport,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare a new CommandSummary for each test case
			cs, cleanUp := prepareTest(t)
			defer func() {
				cleanUp()
			}()
			// Save multiple reports
			err := cs.RecordWithIndex(tc.originalData, tc.summaryIndex)
			assert.NoError(t, err)
			err = cs.RecordWithIndex(tc.originalData, tc.summaryIndex)
			assert.NoError(t, err)
			// Verify file has been saved
			_, nestedFiles, err := cs.getDataFilesPaths()
			assert.NoError(t, err)
			assert.Equal(t, 2, len(nestedFiles[SarifReport]))
		})
	}
}

func TestExtractIndexAndArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []interface{}
		expectedIndex Index
		expectedArgs  []string
	}{
		{"No arguments", nil, "", nil},
		{"Only index", []interface{}{BuildScan}, BuildScan, nil},
		{"Index and args", []interface{}{BuildScan, []string{"arg1", "arg2"}}, BuildScan, []string{"arg1", "arg2"}},
		{"Only args", []interface{}{[]string{"arg1", "arg2"}}, "", []string{"arg1", "arg2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, args := extractIndexAndArgs(tt.args)
			assert.Equal(t, tt.expectedIndex, index)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestDetermineFilePathAndName(t *testing.T) {
	tempDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}()
	tests := []struct {
		name              string
		summaryOutputPath string
		index             Index
		args              []string
		expectedPath      string
		expectedName      string
	}{
		{"No index", tempDir, "", []string{"arg1"}, tempDir, "arg1"},
		{"With index", tempDir, BuildScan, []string{"arg1"}, filepath.Join(tempDir, "build-scans"), "arg1"},
		{"Invalid characters", tempDir, "", []string{"arg1/arg2"}, tempDir, "arg1-arg2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, fileName, err := determineFilePathAndName(tt.summaryOutputPath, tt.index, tt.args)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPath, filePath)
			assert.Equal(t, tt.expectedName, fileName)

		})
	}
}

func TestDetermineFileName(t *testing.T) {
	tests := []struct {
		name         string
		index        Index
		args         []string
		expectedName string
	}{
		{"No arguments", "", nil, "*-data"},
		{"With index", SarifReport, nil, "*.sarif"},
		{"With index and args", BuildScan, []string{"buildName", "buildNumber"}, "buildName-buildNumber"},
		{"With args", "", []string{"arg1", "arg2"}, "arg1-arg2"},
		{"Invalid characters /", "", []string{"arg1/arg2"}, "arg1-arg2"},
		{"Invalid characters :", "", []string{"arg1:arg2"}, "arg1-arg2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := determineFileName(tt.index, tt.args)
			assert.Equal(t, tt.expectedName, fileName)
		})
	}
}

// This function will verify that the actual map contains all the expected keys and sub-keys.
// It will NOT check for key values as they are temp path values, which cannot be predicted.
func verifyCurrentMapping(t *testing.T, expected, actual map[Index]map[string]string) {
	for key, subMap := range expected {
		assert.Contains(t, actual, key, "Key '%s' not found in actual map", key)
		checkSubKeys(t, key, subMap, actual[key])
	}
}

func checkSubKeys(t *testing.T, key Index, expectedSubMap, actualSubMap map[string]string) {
	for subKey, _ := range expectedSubMap {
		if strings.Contains(subKey, "*") {
			assertSubKeyPattern(t, key, subKey, actualSubMap)
		} else {
			assert.Contains(t, actualSubMap, subKey, "Sub-key '%s' not found in actual map for key '%s'", subKey, key)
		}
	}
}

func assertSubKeyPattern(t *testing.T, key Index, subKeyPattern string, actualSubMap map[string]string) {
	found := false
	for actualSubKey := range actualSubMap {
		if match, _ := filepath.Match(subKeyPattern, actualSubKey); match {
			found = true
			break
		}
	}
	assert.True(t, found, "Sub-key pattern '%s' not found in actual map for key '%s'", subKeyPattern, key)
}

func unmarshalData(expected interface{}, filePath string) (interface{}, error) {
	switch expected := expected.(type) {
	case map[string]string:
		var loadedData map[string]string
		err := UnmarshalFromFilePath(filePath, &loadedData)
		return loadedData, err
	case []byte:
		var loadedData []byte
		err := UnmarshalFromFilePath(filePath, &loadedData)
		return loadedData, err
	case string:
		var loadedData string
		err := UnmarshalFromFilePath(filePath, &loadedData)
		return loadedData, err
	case BasicStruct:
		var loadedData BasicStruct
		err := UnmarshalFromFilePath(filePath, &loadedData)
		return loadedData, err
	default:
		return nil, fmt.Errorf("unsupported data type: %T", expected)
	}
}

func prepareTest(t *testing.T) (cs *CommandSummary, cleanUp func()) {
	// Prepare test env
	tempDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	// Set env
	assert.NoError(t, os.Setenv(coreutils.OutputDirPathEnv, tempDir))
	// Create the job summaries home directory
	cs, err = New(&mockCommandSummary{}, "testsCommands")
	assert.NoError(t, err)

	cleanUp = func() {
		assert.NoError(t, os.Unsetenv(coreutils.OutputDirPathEnv))
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}
	return
}
