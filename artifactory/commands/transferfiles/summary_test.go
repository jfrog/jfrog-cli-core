package transferfiles

import (
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCreateErrorsSummaryFile(t *testing.T) {
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	testDataDir := filepath.Join("..", "testdata", "transfer_summary")
	logFiles := []string{filepath.Join(testDataDir, "logs1.json"), filepath.Join(testDataDir, "logs2.json")}

	// Create Errors Summary Csv File from given JSON log files
	createdCsvPath, err := CreateErrorsSummaryCsvFile(logFiles, tmpDir)
	assert.NoError(t, err)
	createdFile, err := os.Open(createdCsvPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, createdFile.Close())
	}()
	actualFileErrors := new([]FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(createdFile, actualFileErrors))

	// Create expected csv file
	expectedFile, err := os.Open(filepath.Join(testDataDir, "logs.csv"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expectedFile.Close())
	}()
	expectedFileErrors := new([]FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(expectedFile, expectedFileErrors))

	if !reflect.DeepEqual(expectedFileErrors, actualFileErrors) {
		t.Errorf("Expected value: %v, got: %v.", expectedFileErrors, actualFileErrors)
	}
}
