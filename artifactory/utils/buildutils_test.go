package utils

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	artclientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var timestamp = strconv.FormatInt(time.Now().Unix(), 10)

func TestGetBuildName(t *testing.T) {
	const buildName = "buildName1"
	const buildNameEnv = "envBuildName"
	const buildNameFile = "fileBuildName"

	// Setup global build name env var.
	// Ensure that other parallel tests won't be effected.
	oldBuildName := coreutils.BuildName
	coreutils.BuildName = oldBuildName + timestamp
	defer func() { coreutils.BuildName = oldBuildName }()

	// Create build config in temp folder.
	tmpDir, err := fileutils.CreateTempDir()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()

	// Create build config in temp folder
	confFileName := filepath.Join(tmpDir, ".jfrog", "projects")
	assert.NoError(t, fileutils.CopyFile(confFileName, filepath.Join("testdata", "build.yaml")))

	wdCopy, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(tmpDir))
	defer func() {
		assert.NoError(t, os.Chdir(wdCopy))
	}()

	buildConfig := NewBuildConfiguration(buildName, "buildNumber", "module", "project")
	for i := 0; i < 2; i++ {
		// Validate build name form params input (first priority).
		actualBuildName, err := buildConfig.GetBuildName()
		assert.NoError(t, err)
		assert.Equal(t, actualBuildName, buildName)

		// Set build name // Set build name using env var.
		// We're now making suer that these env vars aren't affecting the build name and number,
		// because they should still be read from the params.using env var.
		os.Setenv(coreutils.BuildName, buildNameEnv)
	}

	// Validate build name form env var (second priority).
	buildConfig.SetBuildName("")
	actualBuildName, err := buildConfig.GetBuildName()
	assert.NoError(t, err)
	assert.Equal(t, actualBuildName, buildNameEnv)
	assert.NoError(t, os.Unsetenv(coreutils.BuildName))

	// Validate build name form config file (third priority).
	buildConfig.SetBuildName("")
	actualBuildName, err = buildConfig.GetBuildName()
	assert.NoError(t, err)
	assert.Equal(t, buildNameFile, actualBuildName)
}

func TestGetBuildNumber(t *testing.T) {
	const buildNumber = "buildNumber1"
	const buildNumberEnv = "envBuildNumber"
	const buildNumberFromFile = artclientutils.LatestBuildNumberKey

	// Create build config in temp folder.
	tmpDir, err := fileutils.CreateTempDir()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()

	// Create build config in temp folder
	confFileName := filepath.Join(tmpDir, ".jfrog", "projects")
	assert.NoError(t, fileutils.CopyFile(confFileName, filepath.Join("testdata", "build.yaml")))

	wdCopy, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(tmpDir))
	defer func() {
		assert.NoError(t, os.Chdir(wdCopy))
	}()

	// Setup global build number env var.
	// Make sure other parallel tests won't be affected.
	oldBuildNumber := coreutils.BuildNumber
	coreutils.BuildNumber = oldBuildNumber + timestamp
	defer func() { coreutils.BuildNumber = oldBuildNumber }()

	buildConfig := NewBuildConfiguration("", buildNumber, "module", "project")
	for i := 0; i < 2; i++ {
		// Validate build number form params input (first priority).
		actualBuildNumber := buildConfig.GetBuildNumber()
		assert.Equal(t, actualBuildNumber, buildNumber)

		// Set build number using env var.
		os.Setenv(coreutils.BuildNumber, buildNumberEnv)
	}

	// Validate build number form env var (second priority).
	buildConfig.SetBuildNumber("")
	assert.Equal(t, buildConfig.GetBuildNumber(), buildNumberEnv)
	assert.NoError(t, os.Unsetenv(coreutils.BuildNumber))

	// Validate build number form file (third priority).
	buildConfig.SetBuildNumber("")
	assert.Equal(t, buildConfig.GetBuildNumber(), buildNumberFromFile)
}
