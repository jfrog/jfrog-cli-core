package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

type mockCommandSummary struct {
	CommandSummaryInterface
}

type BasicStruct struct {
	Field1 string
	Field2 int
}

func (tcs *mockCommandSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	return "mockMarkdown", nil
}

func TestCommandSummaryFileSystemBehaviour(t *testing.T) {
	cs, cleanUp := prepareTest(t)
	defer func() {
		cleanUp()
	}()

	// Call GenerateMarkdownFromFiles
	err := cs.CreateMarkdown("someData")
	assert.NoError(t, err)

	// Verify that the directory contains two files
	files, err := os.ReadDir(cs.summaryOutputPath)
	assert.NoError(t, err, "Failed to read directory 'test'")
	assert.Equal(t, 2, len(files), "Directory 'test' does not contain exactly two files")

	// Verify a markdown file has been created
	assert.FileExists(t, path.Join(cs.summaryOutputPath, "markdown.md"))
}

func TestDataPersistence(t *testing.T) {
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
			err := cs.saveDataToFileSystem(tc.originalData)
			assert.NoError(t, err)

			// Verify file has been saved
			dataFiles, err := cs.getAllDataFilesPaths()
			assert.NoError(t, err)
			assert.NotEqual(t, 0, len(dataFiles))

			// Verify that data has not been corrupted
			loadedData, err := unmarshalData(tc.originalData, dataFiles[0])
			assert.NoError(t, err)
			assert.EqualValues(t, tc.originalData, loadedData)
		})
	}
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
	assert.NoError(t, os.Setenv(OutputDirPathEnv, tempDir))
	// Create the job summaries home directory
	cs, err = New(&mockCommandSummary{}, "testsCommands")
	assert.NoError(t, err)

	cleanUp = func() {
		assert.NoError(t, os.Unsetenv(OutputDirPathEnv))
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}
	return
}
