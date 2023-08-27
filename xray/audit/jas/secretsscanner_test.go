package jas

import (
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"

	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestNewSecretsScanManager(t *testing.T) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretScanManager := newSecretsScanManager(scanner)

	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.scanner.configFileName)
	assert.NotEmpty(t, secretScanManager.scanner.resultsFileName)
	assert.Equal(t, &fakeServerDetails, secretScanManager.scanner.serverDetails)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretScanManager := newSecretsScanManager(scanner)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = secretScanManager.createConfigFile(currWd)
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(secretScanManager.scanner.configFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(secretScanManager.scanner.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(secretScanManager.scanner.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestRunAnalyzerManager_ReturnsGeneralError(t *testing.T) {
	defer func() {
		os.Clearenv()
	}()

	// Arrange
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretScanManager := newSecretsScanManager(scanner)

	// Act
	err = secretScanManager.runAnalyzerManager()

	// Assert
	assert.Error(t, err)
}

func TestParseResults_EmptyResults(t *testing.T) {
	// Arrange
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretScanManager := newSecretsScanManager(scanner)
	secretScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	secretScanManager.secretsScannerResults, err = getSourceCodeScanResults(secretScanManager.scanner.resultsFileName, scanner.workingDirs[0], false)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretScanManager := newSecretsScanManager(scanner)
	secretScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "contain-secrets.sarif")

	// Act
	secretScanManager.secretsScannerResults, err = getSourceCodeScanResults(secretScanManager.scanner.resultsFileName, scanner.workingDirs[0], false)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 7, len(secretScanManager.secretsScannerResults))
}

func TestGetSecretsScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	secretsResults, err := getSecretsScanResults(scanner)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to run Secrets scan")
	assert.Nil(t, secretsResults)
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
