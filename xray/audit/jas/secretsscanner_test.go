package jas

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretsScanManager(t *testing.T) {
	// Act
	secretScanManager, _, err := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.configFileName)
	assert.NotEmpty(t, secretScanManager.resultsFileName)
	assert.Equal(t, &fakeServerDetails, secretScanManager.serverDetails)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := secretScanManager.createConfigFile()

	defer func() {
		err = os.Remove(secretScanManager.configFileName)
		assert.NoError(t, err)
	}()

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(secretScanManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(secretScanManager.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestRunAnalyzerManager_ReturnsGeneralError(t *testing.T) {
	defer func() {
		os.Clearenv()
		analyzerManagerExecutionError = nil
	}()

	// Arrange
	analyzerManagerExecutionError = errors.New("analyzer manager error")
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := secretScanManager.runAnalyzerManager()

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.Error(t, err)
	assert.Equal(t, analyzerManagerExecutionError.Error(), err.Error())
}

func TestParseResults_EmptyResults(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	err := secretScanManager.setScanResults()

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "contain-secrets.sarif")

	// Act
	err := secretScanManager.setScanResults()

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 8, len(secretScanManager.secretsScannerResults))
}

func TestGetSecretsScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	defer func() {
		analyzerManagerExecutionError = nil
	}()

	// Arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)

	// Act
	secretsResults, entitledForSecrets, err := getSecretsScanResults(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf(secScanFailureMessage, analyzerManagerErrorMessage), err.Error())
	assert.Nil(t, secretsResults)
	assert.True(t, entitledForSecrets)
}

func TestHideSecret(t *testing.T) {
	tests := []struct {
		secret         string
		expectedOutput string
	}{
		{secret: "", expectedOutput: "***"},
		{secret: "12", expectedOutput: "***"},
		{secret: "123", expectedOutput: "***"},
		{secret: "123456789", expectedOutput: "123************"},
		{secret: "3478hfnkjhvd848446gghgfh", expectedOutput: "347************"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, hideSecret(test.secret))
	}
}
