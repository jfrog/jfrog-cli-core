package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretsScanManager(t *testing.T) {
	// Act
	secretScanManager, _, err := newSecretsScanManager(&fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.configFileName)
	assert.NotEmpty(t, secretScanManager.resultsFileName)
	assert.Equal(t, &fakeServerDetails, secretScanManager.serverDetails)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, nil, &analyzerManagerMock{})
	assert.NoError(t, secretsManagerError)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = secretScanManager.createConfigFile(currWd)
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(secretScanManager.configFileName)
		assert.NoError(t, err)
	}()

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
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, nil, &analyzerManagerMock{})

	// Act
	err := secretScanManager.runAnalyzerManager()

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.Error(t, err)
	assert.Equal(t, analyzerManagerExecutionError.Error(), err.Error())
}

func TestParseResults_EmptyResults(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, nil, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = setIacOrSecretsScanResults(secretScanManager.resultsFileName, false)

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, nil, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "contain-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = setIacOrSecretsScanResults(secretScanManager.resultsFileName, false)

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 8, len(secretScanManager.secretsScannerResults))
}

func TestParseResults_ResultsContainSecretsWithWorkingDirs(t *testing.T) {
	// Arrange
	secretScanManager, _, secretsManagerError := newSecretsScanManager(&fakeServerDetails, []string{"secret_generic", "more_secrets"}, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "contain-secrets-multi-wd.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = setIacOrSecretsScanResults(secretScanManager.resultsFileName, false)

	// Assert
	assert.NoError(t, secretsManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 2, len(secretScanManager.secretsScannerResults))
	firstSecretDir := secretScanManager.secretsScannerResults[0].File
	secondSecretDir := secretScanManager.secretsScannerResults[1].File
	assert.Contains(t, firstSecretDir, "more_secrets")
	assert.Contains(t, secondSecretDir, "secret_generic")
}

func TestGetSecretsScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	defer func() {
		analyzerManagerExecutionError = nil
	}()

	// Arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)

	// Act
	secretsResults, entitledForSecrets, err := getSecretsScanResults(&fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "failed to run Secrets scan. Exit code received: analyzer manager failure message", err.Error())
	assert.Nil(t, secretsResults)
	assert.False(t, entitledForSecrets)
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
