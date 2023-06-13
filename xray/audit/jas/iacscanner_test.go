package jas

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewIacScanManager(t *testing.T) {
	// Act
	iacScanManager, _, err := newIacScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager)
	assert.NotEmpty(t, iacScanManager.configFileName)
	assert.NotEmpty(t, iacScanManager.resultsFileName)
	assert.Equal(t, &fakeServerDetails, iacScanManager.serverDetails)
}

func TestIacScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := iacScanManager.createConfigFile()

	defer func() {
		err = os.Remove(iacScanManager.configFileName)
		assert.NoError(t, err)
	}()

	// Assert
	assert.NoError(t, iacManagerError)
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(iacScanManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(iacScanManager.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestIacParseResults_EmptyResults(t *testing.T) {
	// Arrange
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, &analyzerManagerMock{})
	iacScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "no-violations.sarif")

	// Act
	err := iacScanManager.setScanResults()

	// Assert
	assert.NoError(t, iacManagerError)
	assert.NoError(t, err)
	assert.Empty(t, iacScanManager.iacScannerResults)
}

func TestIacParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, &analyzerManagerMock{})
	iacScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "contains-iac-violations.sarif")

	// Act
	err := iacScanManager.setScanResults()

	// Assert
	assert.NoError(t, iacManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager.iacScannerResults)
	assert.Equal(t, 4, len(iacScanManager.iacScannerResults))
}
