package npm

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	testsUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

const minimumWorkspacesNpmVersion = "7.24.2"

func TestNpmPackWorkspaces(t *testing.T) {

	npmVersion, executablePath, err := biutils.GetNpmVersionAndExecPath(nil)
	assert.NoError(t, err)
	// In npm under v7 skip test
	if npmVersion.Compare(minimumWorkspacesNpmVersion) > 0 {
		log.Info("Test skipped as this function in not supported in npm version " + npmVersion.GetVersion())
		return
	}

	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()

	npmProjectPath := filepath.Join("..", "..", "..", "tests", "testdata", "npm-workspaces")
	err = utils.CopyDir(npmProjectPath, tmpDir, true, nil)
	assert.NoError(t, err)

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	chdirCallback := testsUtils.ChangeDirWithCallback(t, cwd, tmpDir)
	defer chdirCallback()

	packedFileNames, err := Pack([]string{"--workspaces", "--verbose"}, executablePath)
	assert.NoError(t, err)

	expected := []string{"module1-1.0.0.tgz", "module2-1.0.0.tgz"}
	assert.Equal(t, expected, packedFileNames)
}

func TestNpmPack(t *testing.T) {

	_, executablePath, err := biutils.GetNpmVersionAndExecPath(nil)
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	npmProjectPath := filepath.Join("..", "..", "..", "tests", "testdata", "npm-workspaces")
	err = utils.CopyDir(npmProjectPath, tmpDir, false, nil)
	assert.NoError(t, err)

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	chdirCallback := testsUtils.ChangeDirWithCallback(t, cwd, tmpDir)
	defer chdirCallback()

	packedFileNames, err := Pack([]string{"--verbose"}, executablePath)
	assert.NoError(t, err)

	expected := []string{"npm-pack-test-1.0.0.tgz"}
	assert.Equal(t, expected, packedFileNames)
}
