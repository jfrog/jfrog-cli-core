package utils

import (
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRemoveDuplicateValues(t *testing.T) {
	tests := []struct {
		testedSlice    []string
		expectedResult []string
	}{
		{testedSlice: []string{"1", "1", "1", "3"}, expectedResult: []string{"1", "3"}},
		{testedSlice: []string{}, expectedResult: []string{}},
		{testedSlice: []string{"1", "2", "3", "4"}, expectedResult: []string{"1", "2", "3", "4"}},
		{testedSlice: []string{"1", "6", "1", "6", "2"}, expectedResult: []string{"1", "6", "2"}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, RemoveDuplicateValues(test.testedSlice))
	}
}

func TestGetSecretFileName_InputIsValid(t *testing.T) {
	// Arrange
	projectRootPath := "Users/user/Desktop/secrets_scanner/"

	// Arrange
	input := "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &input}}},
		},
	}

	// Act
	fileName := ExtractRelativePath(GetResultFileName(secret), projectRootPath)

	// Assert
	assert.Equal(t, "/tests/req.nodejs/file.js", fileName)
}

func TestGetSecretFileName_FileNameIsInvalid(t *testing.T) {
	// Arrange
	projectRootPath := "Users/user/Desktop/secrets_scanner"

	input := "invalid_input"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &input}}},
		},
	}

	// Act
	fileName := ExtractRelativePath(GetResultFileName(secret), projectRootPath)

	// Assert
	assert.Equal(t, input, fileName)
}

func TestGetSecretFileName_FileNameIsMissing(t *testing.T) {
	// Arrange
	projectRootPath := "Users/user/Desktop/secrets_scanner"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: nil}}},
		},
	}

	// Act
	fileName := ExtractRelativePath(GetResultFileName(secret), projectRootPath)

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
	fileName := GetResultLocationInFile(secret)

	// Assert
	assert.Equal(t, "19:25", fileName)
}

func TestGetSeverity_LevelFieldExist(t *testing.T) {
	// Arrange
	levelValue := "High"
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{}}},
		},
		Level: &levelValue,
	}

	// Act
	severity := GetResultSeverity(secret)

	// Assert
	assert.Equal(t, levelValue, severity)
}

func TestGetSeverity_LevelFieldMissing_ShouldReturnDefaultValue(t *testing.T) {
	// Arrange
	secret := &sarif.Result{
		Locations: []*sarif.Location{
			{PhysicalLocation: &sarif.PhysicalLocation{Region: &sarif.Region{}}},
		},
	}

	// Act
	severity := GetResultSeverity(secret)

	// Assert
	assert.Equal(t, "Medium", severity)
}
