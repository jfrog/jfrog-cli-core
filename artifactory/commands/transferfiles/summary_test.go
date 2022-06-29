package transferfiles

import (
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGenerateSummaryFiles(t *testing.T) {
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	testDataDir := filepath.Join("..", "testdata", "transfer_summary")
	logFiles := []string{filepath.Join(testDataDir, "logs1.json"), filepath.Join(testDataDir, "logs2.json")}

	// Create expected file errors
	assert.NoError(t, GenerateSummaryFiles(logFiles, tmpDir))
	expectedFile, err := os.Open(filepath.Join(testDataDir, "logs.csv"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expectedFile.Close())
	}()
	expectedFileErrors := new([]FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(expectedFile, expectedFileErrors))

	// Get actual test errors
	files, err := fileutils.ListFiles(tmpDir, false)
	assert.NoError(t, err)
	actualFile, err := os.Open(files[0])
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, actualFile.Close())
	}()
	actualFileErrors := new([]FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(actualFile, actualFileErrors))
	if !reflect.DeepEqual(expectedFileErrors, actualFileErrors) {
		t.Errorf("Expected value: %v, got: %v.", expectedFileErrors, actualFileErrors)
	}
}
