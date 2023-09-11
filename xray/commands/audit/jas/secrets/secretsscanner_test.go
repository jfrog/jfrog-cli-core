package secrets

import (
	"os"
	"path/filepath"
	"testing"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestNewSecretsScanManager(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	secretScanManager := newSecretsScanManager(scanner)

	assert.NotEmpty(t, secretScanManager)
	assert.NotEmpty(t, secretScanManager.scanner.ConfigFileName)
	assert.NotEmpty(t, secretScanManager.scanner.ResultsFileName)
	assert.Equal(t, &jas.FakeServerDetails, secretScanManager.scanner.ServerDetails)
}

func TestSecretsScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	secretScanManager := newSecretsScanManager(scanner)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = secretScanManager.createConfigFile(jfrogappsconfig.Module{SourceRoot: currWd})
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(secretScanManager.scanner.ConfigFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(secretScanManager.scanner.ConfigFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(secretScanManager.scanner.ConfigFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestRunAnalyzerManager_ReturnsGeneralError(t *testing.T) {
	defer func() {
		os.Clearenv()
	}()

	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	secretScanManager := newSecretsScanManager(scanner)
	assert.Error(t, secretScanManager.runAnalyzerManager())
}

func TestParseResults_EmptyResults(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Arrange
	secretScanManager := newSecretsScanManager(scanner)
	secretScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "secrets-scan", "no-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = jas.ReadJasScanRunsFromFile(secretScanManager.scanner.ResultsFileName, scanner.JFrogAppsConfig.Modules[0].SourceRoot)

	// Assert
	if assert.NoError(t, err) && assert.NotNil(t, secretScanManager.secretsScannerResults) {
		assert.Len(t, secretScanManager.secretsScannerResults, 1)
		assert.Empty(t, secretScanManager.secretsScannerResults[0].Results)
		secretScanManager.secretsScannerResults = processSecretScanRuns(secretScanManager.secretsScannerResults)
		assert.Len(t, secretScanManager.secretsScannerResults, 1)
		assert.Empty(t, secretScanManager.secretsScannerResults[0].Results)
	}

}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	secretScanManager := newSecretsScanManager(scanner)
	secretScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "secrets-scan", "contain-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = jas.ReadJasScanRunsFromFile(secretScanManager.scanner.ResultsFileName, scanner.JFrogAppsConfig.Modules[0].SourceRoot)

	// Assert
	if assert.NoError(t, err) && assert.NotNil(t, secretScanManager.secretsScannerResults) {
		assert.Len(t, secretScanManager.secretsScannerResults, 1)
		assert.NotEmpty(t, secretScanManager.secretsScannerResults[0].Results)
		secretScanManager.secretsScannerResults = processSecretScanRuns(secretScanManager.secretsScannerResults)
		assert.Len(t, secretScanManager.secretsScannerResults, 1)
		assert.Len(t, secretScanManager.secretsScannerResults[0].Results, 7)
	}
	assert.NoError(t, err)

}

func TestGetSecretsScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	secretsResults, err := RunSecretsScan(scanner)

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
		assert.Equal(t, test.expectedOutput, maskSecret(test.secret))
	}
}
