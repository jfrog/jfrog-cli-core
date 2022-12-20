package python

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
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

func TestPoetryCommandCleanup(t *testing.T) {
	poetryProjectPath, cleanUp := initPoetryTest(t)
	defer cleanUp()

	assert.NoError(t, exec.Command("python", "-m", "pip", "uninstall", "virtualenv", "-y").Run())
	defer func() {
		assert.NoError(t, exec.Command("python", "-m", "pip", "install", "virtualenv").Run())
	}()

	pc := NewPoetryCommand()
	dummyRepoURL := "https://ecosysjfrog.jfrog.io/"
	err := pc.configPoetryRepo(dummyRepoURL, "", "")
	assert.NoError(t, err)

	pyProjectPath := filepath.Join(poetryProjectPath, "pyproject.toml")
	content, err := fileutils.ReadFile(pyProjectPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), dummyRepoURL)

	assert.NoError(t, pc.cleanup())

	content, err = fileutils.ReadFile(pyProjectPath)
	assert.NoError(t, err)
	assert.NotContains(t, string(content), dummyRepoURL)
}

func initPoetryTest(t *testing.T) (string, func()) {
	// Create and change directory to test workspace
	testAbs, err := filepath.Abs(filepath.Join("..", "..", "..", "xray", "commands", "testdata", "poetry-project"))
	assert.NoError(t, err)
	poetryProjectPath, cleanUp := audit.CreateTestWorkspace(t, "poetry-project")
	assert.NoError(t, fileutils.CopyDir(testAbs, poetryProjectPath, true, nil))
	return poetryProjectPath, cleanUp
}
