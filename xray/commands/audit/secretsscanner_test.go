package audit

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretsScanManager_InputIsValid(t *testing.T) {
	// Act
	secretScanManager, _, err := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.configFileName)
	assert.NotEmpty(t, secretScanManager.resultsFileName)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	secretScanManager, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := secretScanManager.createConfigFile()

	// Assert
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(secretScanManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, _ := os.ReadFile(secretScanManager.configFileName)
	assert.True(t, len(fileContent) > 0)

	// Cleanup
	err = os.Remove(secretScanManager.configFileName)
	assert.NoError(t, err)
}

func TestRunAnalyzerManager_ReturnsGeneralError(t *testing.T) {
	// Arrange
	analyzerManagerExecutionError = errors.New("analyzer manager error")
	secretScanManager, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := secretScanManager.runAnalyzerManager()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, analyzerManagerExecutionError.Error(), err.Error())

	// Cleanup
	os.Clearenv()
	analyzerManagerExecutionError = nil
}

func TestParseResults_EmptyResults(t *testing.T) {
	// Arrange
	secretScanManager, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	err := secretScanManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	secretScanManager, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "testdata", "secrets-scan", "contain-secrets.sarif")

	// Act
	err := secretScanManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 8, len(secretScanManager.secretsScannerResults))
}

func TestGetSecretsScan_ExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	// Arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)

	// Act
	secretsResults, entitledForSecrets, err := getSecretsScanResults(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf(secretsScanFailureMessage, analyzerManagerErrorMessage), err.Error())
	assert.Nil(t, secretsResults)
	assert.True(t, entitledForSecrets)

	// Cleanup
	analyzerManagerExecutionError = nil
}

func TestPartiallyHideSecret_SecretIsEmpty(t *testing.T) {
	// Arrange
	secretScanner, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	hiddenSecret := secretScanner.hideSecret("")

	// Assert
	assert.Equal(t, "***", hiddenSecret)
}

func TestPartiallyHideSecret_SecretIsShorterThanSevenDigits(t *testing.T) {
	// Arrange
	secretScanner, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	hiddenSecret := secretScanner.hideSecret("123")

	// Assert
	assert.Equal(t, "***", hiddenSecret)
}

func TestPartiallyHideSecret_SecretIsLongerThanSevenDigits(t *testing.T) {
	// Arrange
	secretScanner, _, _ := newSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	hiddenSecret := secretScanner.hideSecret("long_secret")

	// Assert
	assert.Equal(t, "lon************", hiddenSecret)
}
