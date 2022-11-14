package transferfiles

import (
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCreateErrorsSummaryFile(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	testDataDir := filepath.Join("..", "testdata", "transfer_summary")
	logFiles := []string{filepath.Join(testDataDir, "logs1.json"), filepath.Join(testDataDir, "logs2.json")}

	// Create Errors Summary Csv File from given JSON log files
	createdCsvPath, err := createErrorsSummaryCsvFile(logFiles, time.Now())
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCsvPath)
	createdFile, err := os.Open(createdCsvPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, createdFile.Close())
	}()
	actualFileErrors := new([]api.FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(createdFile, actualFileErrors))

	// Create expected csv file
	expectedFile, err := os.Open(filepath.Join(testDataDir, "logs.csv"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expectedFile.Close())
	}()
	expectedFileErrors := new([]api.FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(expectedFile, expectedFileErrors))

	if !reflect.DeepEqual(expectedFileErrors, actualFileErrors) {
		t.Errorf("Expected value: %v, got: %v.", expectedFileErrors, actualFileErrors)
	}
}
