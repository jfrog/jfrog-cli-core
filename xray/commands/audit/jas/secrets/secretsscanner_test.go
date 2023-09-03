package secrets

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
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
	err = secretScanManager.createConfigFile(currWd)
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
	secretScanManager.scanner.ResultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "no-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = jas.GetSourceCodeScanResults(secretScanManager.scanner.ResultsFileName, scanner.WorkingDirs[0], utils.Secrets)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, secretScanManager.secretsScannerResults)
}

func TestParseResults_ResultsContainSecrets(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	secretScanManager := newSecretsScanManager(scanner)
	secretScanManager.scanner.ResultsFileName = filepath.Join("..", "..", "commands", "testdata", "secrets-scan", "contain-secrets.sarif")

	// Act
	var err error
	secretScanManager.secretsScannerResults, err = jas.GetSourceCodeScanResults(secretScanManager.scanner.ResultsFileName, scanner.WorkingDirs[0], utils.Secrets)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, secretScanManager.secretsScannerResults)
	assert.Equal(t, 7, len(secretScanManager.secretsScannerResults))
}

func TestGetSecretsScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	secretsResults, err := RunSecretsScan(scanner)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to run Secrets scan")
	assert.Nil(t, secretsResults)
}
