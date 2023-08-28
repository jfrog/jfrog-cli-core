package jas

import (
	"os"
	"path/filepath"
	"testing"

	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewIacScanManager(t *testing.T) {
	// Act
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner([]string{"currentDir"}, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	iacScanManager := newIacScanManager(scanner)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager)
	assert.NotEmpty(t, iacScanManager.scanner.configFileName)
	assert.NotEmpty(t, iacScanManager.scanner.resultsFileName)
	assert.NotEmpty(t, iacScanManager.scanner.workingDirs)
	assert.Equal(t, &fakeServerDetails, iacScanManager.scanner.serverDetails)
}

func TestIacScan_CreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner([]string{"currentDir"}, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	iacScanManager := newIacScanManager(scanner)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = iacScanManager.createConfigFile(currWd)

	defer func() {
		err = os.Remove(iacScanManager.scanner.configFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(iacScanManager.scanner.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(iacScanManager.scanner.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestIacParseResults_EmptyResults(t *testing.T) {
	// Arrange
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	iacScanManager := newIacScanManager(scanner)
	iacScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "no-violations.sarif")

	// Act
	iacScanManager.iacScannerResults, err = getSourceCodeScanResults(iacScanManager.scanner.resultsFileName, scanner.workingDirs[0], utils.IaC)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, iacScanManager.iacScannerResults)
}

func TestIacParseResults_ResultsContainIacViolations(t *testing.T) {
	// Arrange
	scanner, err := NewAdvancedSecurityScanner(nil, &fakeServerDetails)
	assert.NoError(t, err)
	defer func() {
		if scanner.scannerDirCleanupFunc != nil {
			assert.NoError(t, scanner.scannerDirCleanupFunc())
		}
	}()
	iacScanManager := newIacScanManager(scanner)
	iacScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "iac-scan", "contains-iac-violations.sarif")

	// Act
	iacScanManager.iacScannerResults, err = getSourceCodeScanResults(iacScanManager.scanner.resultsFileName, scanner.workingDirs[0], utils.IaC)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, iacScanManager.iacScannerResults)
	assert.Equal(t, 4, len(iacScanManager.iacScannerResults))
}
