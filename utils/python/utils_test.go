package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func TestAddRepoToPyprojectFile(t *testing.T) {
	poetryProjectPath, cleanUp := initPoetryTest(t)
	defer cleanUp()
	pyProjectPath := filepath.Join(poetryProjectPath, "pyproject.toml")
	dummyRepoName := "test-repo-name"
	dummyRepoURL := "https://ecosysjfrog.jfrog.io/"

	err := addRepoToPyprojectFile(pyProjectPath, dummyRepoName, dummyRepoURL)
	assert.NoError(t, err)
	// Validate pyproject.toml file content
	content, err := fileutils.ReadFile(pyProjectPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), dummyRepoURL)
}

func initPoetryTest(t *testing.T) (string, func()) {
	// Create and change directory to test workspace
	testAbs, err := filepath.Abs(filepath.Join("..", "..", "tests", "testdata", "poetry-project"))
	assert.NoError(t, err)
	poetryProjectPath, cleanUp := tests.CreateTestWorkspace(t, testAbs)
	return poetryProjectPath, cleanUp
}

func TestGetPypiRepoUrlWithCredentials(t *testing.T) {
	tests := []struct {
		name        string
		curationCmd bool
	}{
		{
			name:        "test curation command true",
			curationCmd: true,
		},
		{
			name:        "test curation command false",
			curationCmd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _, _, err := GetPypiRepoUrlWithCredentials(&config.ServerDetails{}, "test", tt.curationCmd)
			require.NoError(t, err)
			assert.Equal(t, tt.curationCmd, strings.Contains(url.Path, coreutils.CurationPassThroughApi))
		})
	}
}
