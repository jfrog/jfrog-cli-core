package jobsummaries

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatSummaryMarkdownBaseImpl(t *testing.T) {
	// Prepare test env
	tempDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(HomeDirPathEnv, tempDir))
	assert.NoError(t, os.Setenv(githubActionsEnv, "true"))
	defer func() {
		assert.NoError(t, os.Unsetenv(HomeDirPathEnv))
		assert.NoError(t, os.Unsetenv(githubActionsEnv))
		assert.NoError(t, fileutils.RemoveTempDir(tempDir))
	}()
	// Create the job summaries home directory
	_, err = prepareFileSystem()
	assert.NoError(t, err)

	// Mock appendObjectsFunc
	mockAppendObjectsFunc := func(content interface{}, previousObjects []byte) ([]byte, error) {
		return []byte("mockData"), nil
	}
	// Mock generateMarkdownFunc
	mockGenerateMarkdownFunc := func(dataAsBytes []byte) (string, error) {
		return "mockMarkdown", nil
	}

	// Set up test data
	testData := "testData"
	testSection := MarkdownSection("testSection")

	// Call CreatSummaryMarkdownBaseImpl
	err = CreatSummaryMarkdownBaseImpl(testData, testSection, mockAppendObjectsFunc, mockGenerateMarkdownFunc)
	assert.NoError(t, err)

	// Check if the data file was created and contains the expected data
	data, err := loadFile(getSectionDataFileName(testSection))
	assert.NoError(t, err)
	assert.Equal(t, "mockData", string(data))

	// Check if the markdown file was created and contains the expected markdown
	markdown, err := loadFile(string(testSection) + ".md")
	assert.NoError(t, err)
	assert.Equal(t, "mockMarkdown", string(markdown))
}

func TestGetJobSummariesHomeDirPath(t *testing.T) {
	basePath := "/tmp"
	err := os.Setenv(HomeDirPathEnv, basePath)
	defer func() {
		assert.NoError(t, os.Unsetenv(HomeDirPathEnv))
	}()
	assert.NoError(t, err)

	homeDir, err := GetJobSummariesHomeDirPath()
	assert.NoError(t, err)

	expected := filepath.Join(basePath, JobSummaryDirName)
	assert.Equal(t, expected, homeDir)
}
