package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewIacScanManager(t *testing.T) {
	// Act
	iacScanManager, _, err := newIacScanManager(&fakeServerDetails, []string{"currentDir"}, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager)
	assert.NotEmpty(t, iacScanManager.configFileName)
	assert.NotEmpty(t, iacScanManager.resultsFileName)
	assert.NotEmpty(t, iacScanManager.workingDirs)
	assert.Equal(t, &fakeServerDetails, iacScanManager.serverDetails)
}

func TestIacScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, []string{"currentDir"}, &analyzerManagerMock{})
	assert.NoError(t, iacManagerError)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = iacScanManager.createConfigFile(currWd)

	defer func() {
		err = os.Remove(iacScanManager.configFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(iacScanManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(iacScanManager.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestIacParseResults_EmptyResults(t *testing.T) {
	// Arrange
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(nil)
	assert.NoError(t, err)
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, fullPathWorkingDirs, &analyzerManagerMock{})
	iacScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "no-violations.sarif")

	// Act
	iacScanManager.iacScannerResults, err = getIacOrSecretsScanResults(iacScanManager.resultsFileName, fullPathWorkingDirs[0], false)

	// Assert
	assert.NoError(t, iacManagerError)
	assert.NoError(t, err)
	assert.Empty(t, iacScanManager.iacScannerResults)
}

func TestIacParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(nil)
	assert.NoError(t, err)
	iacScanManager, _, iacManagerError := newIacScanManager(&fakeServerDetails, fullPathWorkingDirs, &analyzerManagerMock{})
	iacScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "contains-iac-violations.sarif")

	// Act
	iacScanManager.iacScannerResults, err = getIacOrSecretsScanResults(iacScanManager.resultsFileName, fullPathWorkingDirs[0], false)

	// Assert
	assert.NoError(t, iacManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager.iacScannerResults)
	assert.Equal(t, 4, len(iacScanManager.iacScannerResults))
}
