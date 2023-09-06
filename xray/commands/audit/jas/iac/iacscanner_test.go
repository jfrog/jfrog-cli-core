package iac

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewIacScanManager(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t, "currentDir")
	defer cleanUp()
	// Act
	iacScanManager := newIacScanManager(scanner)

	// Assert
	if assert.NotNil(t, iacScanManager) {
		assert.NotEmpty(t, iacScanManager.scanner.ConfigFileName)
		assert.NotEmpty(t, iacScanManager.scanner.ResultsFileName)
		assert.NotEmpty(t, iacScanManager.scanner.WorkingDirs)
		assert.Equal(t, &jas.FakeServerDetails, iacScanManager.scanner.ServerDetails)
	}
}

func TestIacScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t, "currentDir")
	defer cleanUp()

	iacScanManager := newIacScanManager(scanner)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = iacScanManager.createConfigFile(currWd)

	defer func() {
		err = os.Remove(iacScanManager.scanner.ConfigFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(iacScanManager.scanner.ConfigFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(iacScanManager.scanner.ConfigFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestIacParseResults_EmptyResults(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	// Arrange
	iacScanManager := newIacScanManager(scanner)
	iacScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "iac-scan", "no-violations.sarif")

	// Act
	var err error
	iacScanManager.iacScannerResults, err = utils.ReadScanRunsFromFile(iacScanManager.scanner.ResultsFileName)
	if assert.NoError(t, err) {
		assert.Empty(t, iacScanManager.iacScannerResults)
		processIacScanResults(iacScanManager.iacScannerResults, scanner.WorkingDirs[0])
		assert.Empty(t, iacScanManager.iacScannerResults)
	}
}

func TestIacParseResults_ResultsContainIacViolations(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Arrange
	iacScanManager := newIacScanManager(scanner)
	iacScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "iac-scan", "contains-iac-violations.sarif")

	// Act
	var err error
	iacScanManager.iacScannerResults, err = utils.ReadScanRunsFromFile(iacScanManager.scanner.ResultsFileName)
	if assert.NoError(t, err) {
		assert.Empty(t, iacScanManager.iacScannerResults)
		processIacScanResults(iacScanManager.iacScannerResults, scanner.WorkingDirs[0])
		assert.Equal(t, 4, len(iacScanManager.iacScannerResults))
	}
}
