package jas

import (
	"errors"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretsScanManager_InputIsValid(t *testing.T) {
	// Act
	secretScanManager, err := NewsSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.configFileName)
	assert.NotEmpty(t, secretScanManager.resultsFileName)
}

func TestNewSecretsScanManager_ServerNotValid(t *testing.T) {
	// Act
	secretScanManager, err := NewsSecretsScanManager(nil, &analyzerManagerMock{})

	// Assert
	assert.Nil(t, secretScanManager)
	assert.Error(t, err)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	secretScanManager, _ := NewsSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

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
	secretScanManager, _ := NewsSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})

	// Act
	entitled, err := secretScanManager.runAnalyzerManager()

	// Assert
	assert.True(t, entitled)
	assert.Error(t, err)

	// Cleanup
	os.Clearenv()
	analyzerManagerExecutionError = nil
}

func TestParseResults_EmptyResults(t *testing.T) {
	// Arrange
	secretScanManager, _ := NewsSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "..", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	err := secretScanManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	secretScanManager, _ := NewsSecretsScanManager(&fakeServerDetails, &analyzerManagerMock{})
	secretScanManager.resultsFileName = filepath.Join("..", "..", "..", "testdata", "secrets-scan", "contain-secrets.sarif")

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
	assert.Equal(t, analyzerManagerErrorMessage, err.Error())
	assert.Nil(t, secretsResults)
	assert.True(t, entitledForSecrets)

	// Cleanup
	analyzerManagerExecutionError = nil
}

func TestGetSecretFileName_InputIsValid(t *testing.T) {
	// Arrange
	input := "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &input}}},
		},
	}

	// Act
	fileName := getSecretFileName(secret)

	// Assert
	assert.Equal(t, "Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js", fileName)
}

func TestGetSecretFileName_FileNameIsInvalid(t *testing.T) {
	// Arrange
	input := "invalid_input"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &input}}},
		},
	}

	// Act
	fileName := getSecretFileName(secret)

	// Assert
	assert.Equal(t, input, fileName)
}

func TestGetSecretFileName_FileNameIsMissing(t *testing.T) {
	// Arrange
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: nil}}},
		},
	}

	// Act
	fileName := getSecretFileName(secret)

	// Assert
	assert.Equal(t, "", fileName)
}

func TestGetSecretLocation_InputIsValid(t *testing.T) {
	// Arrange
	startLine := 19
	startColumn := 25
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{
				StartLine:   &startLine,
				StartColumn: &startColumn,
			}}},
		},
	}

	// Act
	fileName := getSecretLocation(secret)

	// Assert
	assert.Equal(t, "19:25", fileName)
}

func TestPartiallyHideSecret_SecretIsEmpty(t *testing.T) {
	// Act
	hiddenSecret := getHiddenSecret("")

	// Assert
	assert.Equal(t, "", hiddenSecret)
}

func TestPartiallyHideSecret_SecretIsShorterThanSevenDigits(t *testing.T) {
	// Act
	hiddenSecret := getHiddenSecret("secret")

	// Assert
	assert.Equal(t, "******", hiddenSecret)
}

func TestPartiallyHideSecret_SecretIsLongerThanSevenDigits(t *testing.T) {
	// Act
	hiddenSecret := getHiddenSecret("long_secret")

	// Assert
	assert.Equal(t, "long_se***********************", hiddenSecret)
}
