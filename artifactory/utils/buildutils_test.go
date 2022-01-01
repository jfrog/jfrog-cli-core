package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	artclientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

var timestamp = strconv.FormatInt(time.Now().Unix(), 10)

const buildNameFile = "fileBuildName"

func TestGetBuildName(t *testing.T) {
	const buildName = "buildName1"
	const buildNameEnv = "envBuildName"

	// Setup global build name env var.
	// Ensure that other parallel tests won't be effected.
	oldBuildName := coreutils.BuildName
	coreutils.BuildName = oldBuildName + timestamp
	defer func() { coreutils.BuildName = oldBuildName }()

	// Create build config in temp folder.
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()

	// Create build config in temp folder
	confFileName := filepath.Join(tmpDir, ".jfrog", "projects")
	assert.NoError(t, fileutils.CopyFile(confFileName, filepath.Join("testdata", "build.yaml")))

	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallBack := testsutils.ChangeDirWithCallback(t, wd, tmpDir)
	defer chdirCallBack()

	buildConfig := NewBuildConfiguration(buildName, "buildNumber", "module", "project")
	for i := 0; i < 2; i++ {
		// Validate build name form params input (first priority).
		actualBuildName, err := buildConfig.GetBuildName()
		assert.NoError(t, err)
		assert.Equal(t, actualBuildName, buildName)

		// Set build name // Set build name using env var.
		// We're now making suer that these env vars aren't affecting the build name and number,
		// because they should still be read from the params.using env var.
		testsutils.SetEnvAndAssert(t, coreutils.BuildName, buildNameEnv)
	}

	// Validate build name form env var (second priority).
	buildConfig.SetBuildName("")
	actualBuildName, err := buildConfig.GetBuildName()
	assert.NoError(t, err)
	assert.Equal(t, actualBuildName, buildNameEnv)
	testsutils.UnSetEnvAndAssert(t, coreutils.BuildName)

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
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()

	// Create build config in temp folder
	confFileName := filepath.Join(tmpDir, ".jfrog", "projects")
	assert.NoError(t, fileutils.CopyFile(confFileName, filepath.Join("testdata", "build.yaml")))

	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallBack := testsutils.ChangeDirWithCallback(t, wd, tmpDir)
	defer chdirCallBack()

	// Setup global build number env var.
	// Make sure other parallel tests won't be affected.
	oldBuildNumber := coreutils.BuildNumber
	coreutils.BuildNumber = oldBuildNumber + timestamp
	defer func() { coreutils.BuildNumber = oldBuildNumber }()

	buildConfig := NewBuildConfiguration("", buildNumber, "module", "project")
	for i := 0; i < 2; i++ {
		// Validate build number form params input (first priority).
		actualBuildNumber, err := buildConfig.GetBuildNumber()
		assert.NoError(t, err)
		assert.Equal(t, actualBuildNumber, buildNumber)

		// Set build number using env var.
		testsutils.SetEnvAndAssert(t, coreutils.BuildNumber, buildNumberEnv)
	}

	// Validate build number form env var (second priority).
	buildConfig.SetBuildNumber("")
	actualBuildNumber, err := buildConfig.GetBuildNumber()
	assert.NoError(t, err)
	assert.Equal(t, actualBuildNumber, buildNumberEnv)
	testsutils.UnSetEnvAndAssert(t, coreutils.BuildNumber)

	// Validate build number form file (third priority).
	buildConfig.SetBuildNumber("")
	actualBuildNumber, err = buildConfig.GetBuildNumber()
	assert.NoError(t, err)
	assert.Equal(t, actualBuildNumber, buildNumberFromFile)
}

func TestGetProject(t *testing.T) {
	const project = "project1"
	const projectEnv = "envProject"

	// Setup global project env var.
	// Make sure other parallel tests won't be affected.
	oldProject := coreutils.Project
	coreutils.Project = oldProject + timestamp
	defer func() { coreutils.Project = oldProject }()

	buildConfig := NewBuildConfiguration("", "", "", project)
	for i := 0; i < 2; i++ {
		actualProject := buildConfig.GetProject()
		assert.Equal(t, actualProject, project)

		// Set project using env var.
		testsutils.SetEnvAndAssert(t, coreutils.Project, projectEnv)
	}

	// Validate project form env var (second priority).
	buildConfig.SetProject("")
	actualProject := buildConfig.GetProject()
	assert.Equal(t, actualProject, projectEnv)
	testsutils.UnSetEnvAndAssert(t, coreutils.Project)
}

func TestIsCollectBuildInfo(t *testing.T) {
	buildConfig := NewBuildConfiguration("", "", "", "")
	toCollect, err := buildConfig.IsCollectBuildInfo()
	assert.NoError(t, err)
	assert.False(t, toCollect)
	buildConfig.SetBuildName("a")
	toCollect, err = buildConfig.IsCollectBuildInfo()
	assert.NoError(t, err)
	assert.False(t, toCollect)
	buildConfig.SetProject("a")
	toCollect, err = buildConfig.IsCollectBuildInfo()
	assert.NoError(t, err)
	assert.False(t, toCollect)
	buildConfig.SetModule("a")
	toCollect, err = buildConfig.IsCollectBuildInfo()
	assert.NoError(t, err)
	assert.False(t, toCollect)
	buildConfig.SetBuildNumber("a")
	toCollect, err = buildConfig.IsCollectBuildInfo()
	assert.NoError(t, err)
	assert.True(t, toCollect)
}

func TestIsLoadedFromConfigFile(t *testing.T) {
	// Create build config in temp folder.
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	buildConfig := NewBuildConfiguration("", "", "", "")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())
	buildConfig.SetBuildName("a")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())
	buildConfig.SetProject("a")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())
	buildConfig.SetModule("a")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())
	buildConfig.SetBuildNumber("a")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())

	buildConfig.SetBuildNumber("")
	buildConfig.SetBuildName("")
	// Create build config in temp folder
	confFileName := filepath.Join(tmpDir, ".jfrog", "projects")
	assert.NoError(t, fileutils.CopyFile(confFileName, filepath.Join("testdata", "build.yaml")))
	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallBack := testsutils.ChangeDirWithCallback(t, wd, tmpDir)
	defer chdirCallBack()
	buildName, err := buildConfig.GetBuildName()
	assert.NoError(t, err)
	assert.True(t, buildConfig.IsLoadedFromConfigFile())
	assert.Equal(t, buildName, buildNameFile)
	buildNumber, err := buildConfig.GetBuildNumber()
	assert.NoError(t, err)
	assert.Equal(t, buildNumber, artclientutils.LatestBuildNumberKey)
	assert.True(t, buildConfig.IsLoadedFromConfigFile())

	// Try to get build number first before build name.
	buildConfig = NewBuildConfiguration("", "", "", "")
	assert.False(t, buildConfig.IsLoadedFromConfigFile())

	// Create build config in temp folder
	buildNumber, err = buildConfig.GetBuildNumber()
	assert.NoError(t, err)
	assert.True(t, buildConfig.IsLoadedFromConfigFile())
	buildName, err = buildConfig.GetBuildName()
	assert.True(t, buildConfig.IsLoadedFromConfigFile())
	assert.NoError(t, err)
	assert.Equal(t, buildName, buildNameFile)
	assert.Equal(t, buildNumber, artclientutils.LatestBuildNumberKey)
}
