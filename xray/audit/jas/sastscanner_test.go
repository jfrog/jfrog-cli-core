package jas

import (
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
)


func TestNewSastScanManager(t *testing.T) {
	scanner, cleanUp := initJasTest(t, "currentDir")
	defer cleanUp()
	// Act
	sastScanManager := newSastScanManager(scanner)

	// Assert
	if assert.NotNil(t, sastScanManager) {
		assert.NotEmpty(t, sastScanManager.scanner.configFileName)
		assert.NotEmpty(t, sastScanManager.scanner.resultsFileName)
		assert.NotEmpty(t, sastScanManager.scanner.workingDirs)
		assert.Equal(t, &fakeServerDetails, sastScanManager.scanner.serverDetails)
	}
}

func TestSastParseResults_EmptyResults(t *testing.T) {
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	// Arrange
	sastScanManager := newSastScanManager(scanner)
	sastScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "sast-scan", "no-violations.sarif")

	// Act
	var err error
	sastScanManager.sastScannerResults, err = getSourceCodeScanResults(sastScanManager.scanner.resultsFileName, scanner.workingDirs[0], utils.Sast)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, sastScanManager.sastScannerResults)
}

func TestSastParseResults_ResultsContainIacViolations(t *testing.T) {
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	// Arrange
	sastScanManager := newSastScanManager(scanner)
	sastScanManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "sast-scan", "contains-sast-violations.sarif")

	// Act
	var err error
	sastScanManager.sastScannerResults, err = getSourceCodeScanResults(sastScanManager.scanner.resultsFileName, scanner.workingDirs[0], utils.Sast)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, sastScanManager.sastScannerResults)
	// File has 4 results, 2 of them at the same location different codeFlow
	assert.Equal(t, 3, len(sastScanManager.sastScannerResults))
}