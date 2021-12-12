package gradleutils

import (
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestDownloadExtractorsFromReleases(t *testing.T) {
	// Set 'JFROG_CLI_DEPENDENCIES_DIR' env var to a temp dir
	tempDirPath, createTempDirCallback := fileutils.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	tests.SetEnvAndAssert(t, coreutils.DependenciesDir, tempDirPath)
	// Make sure the JAR will be downloaded from releases.jfrog.io
	tests.UnSetEnvAndAssert(t, utils.ExtractorsRemoteEnv)

	// Download JAR
	dependenciesPath, gradlePluginFilename, err := downloadGradleDependencies()
	assert.NoError(t, err)

	// Make sure the Gradle build-info extractor JAR exist
	expectedJarPath := filepath.Join(dependenciesPath, gradlePluginFilename)
	assert.FileExists(t, expectedJarPath)
}
