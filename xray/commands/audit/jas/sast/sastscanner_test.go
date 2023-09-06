package sast

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewSastScanManager(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t, "currentDir")
	defer cleanUp()
	// Act
	sastScanManager := newSastScanManager(scanner)

	// Assert
	if assert.NotNil(t, sastScanManager) {
		assert.NotEmpty(t, sastScanManager.scanner.ConfigFileName)
		assert.NotEmpty(t, sastScanManager.scanner.ResultsFileName)
		assert.NotEmpty(t, sastScanManager.scanner.WorkingDirs)
		assert.Equal(t, &jas.FakeServerDetails, sastScanManager.scanner.ServerDetails)
	}
}

func TestSastParseResults_EmptyResults(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	// Arrange
	sastScanManager := newSastScanManager(scanner)
	sastScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "sast-scan", "no-violations.sarif")

	// Act
	var err error
	sastScanManager.sastScannerResults, err = utils.ReadScanRunsFromFile(sastScanManager.scanner.ResultsFileName)

	// Assert
	if assert.NoError(t, err) {
		assert.Empty(t, sastScanManager.sastScannerResults)
		processSastScanResults(sastScanManager.sastScannerResults, scanner.WorkingDirs[0])
		assert.Empty(t, sastScanManager.sastScannerResults)
	}
}

func TestSastParseResults_ResultsContainIacViolations(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Arrange
	sastScanManager := newSastScanManager(scanner)
	sastScanManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "sast-scan", "contains-sast-violations.sarif")

	// Act
	var err error
	sastScanManager.sastScannerResults, err = utils.ReadScanRunsFromFile(sastScanManager.scanner.ResultsFileName)

	// Assert
	if assert.NoError(t, err) {
		assert.NotEmpty(t, sastScanManager.sastScannerResults)
		processSastScanResults(sastScanManager.sastScannerResults, scanner.WorkingDirs[0])
		// File has 4 results, 2 of them at the same location different codeFlow
		assert.Equal(t, 3, len(sastScanManager.sastScannerResults))
	}	
}
