package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandSummaryFileSystemBehaviour(t *testing.T) {
	// Prepare test env
	tempDir, err := fileutils.CreateTempDir()
	defer func() {
		err = fileutils.RemoveTempDir(tempDir)
	}()
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(OutputDirPathEnv, tempDir))
	defer func() {
		assert.NoError(t, os.Unsetenv(OutputDirPathEnv))
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}()
	// Create the job summaries home directory
	err = prepareFileSystem()
	assert.NoError(t, err)

	// Mock generateMarkdownFunc
	mockGenerateMarkdownFunc := func(filePaths []string) (string, error) {
		return "mockMarkdown", nil
	}

	subDirName := "testCommand"
	// Call CreateMarkdown
	err = CreateMarkdown("someData", subDirName, mockGenerateMarkdownFunc)
	assert.NoError(t, err)

	// Verify that a directory subDirName exists
	testDir := filepath.Join(getOutputDirPath(), subDirName)
	_, err = os.Stat(testDir)
	assert.NoError(t, err, "Directory 'test' does not exist")

	// Verify that the directory contains two files
	files, err := os.ReadDir(testDir)
	assert.NoError(t, err, "Failed to read directory 'test'")
	assert.Equal(t, 2, len(files), "Directory 'test' does not contain exactly two files")

	// Verify that one of the files is named "markdown.md"
	var containsMarkdown bool
	for _, file := range files {
		if file.Name() == "markdown.md" {
			containsMarkdown = true
			break
		}
	}
	assert.True(t, containsMarkdown, "File 'markdown.md' does not exist in the sub directory")

}

func TestGetJobSummariesHomeDirPath(t *testing.T) {
	basePath := "/tmp"
	err := os.Setenv(OutputDirPathEnv, basePath)
	defer func() {
		assert.NoError(t, os.Unsetenv(OutputDirPathEnv))
	}()
	assert.NoError(t, err)

	expected := filepath.Join(basePath, OutputDirName)
	assert.Equal(t, expected, getOutputDirPath())
}

// Tests the saves & loading different types of objects.
func TestDataHandle(t *testing.T) {
	// Prepare test environment
	tempDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}()
	assert.NoError(t, os.Setenv(OutputDirPathEnv, tempDir))
	defer func() {
		assert.NoError(t, os.Unsetenv(OutputDirPathEnv))
	}()

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
			// Save data to file
			err = saveDataToFileSystem(tc.originalData, tc.dirName)
			assert.NoError(t, err)

			// Load data from file
			files, err := os.ReadDir(filepath.Join(getOutputDirPath(), tc.dirName))
			assert.NoError(t, err)
			assert.NotEqual(t, 0, len(files))

			loadedData, err := unmarshalData(tc.originalData, filepath.Join(getOutputDirPath(), tc.dirName, files[0].Name()))
			assert.NoError(t, err)
			assert.Equal(t, tc.originalData, loadedData)
		})
	}
}

// Define a basic struct
type BasicStruct struct {
	Field1 string
	Field2 int
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
