package gradle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func TestDownloadExtractorsFromOjo(t *testing.T) {
	// Set 'JFROG_CLI_DEPENDENCIES_DIR' env var to a temp dir
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer fileutils.RemoveTempDir(tempDirPath)
	err = os.Setenv(coreutils.DependenciesDir, tempDirPath)
	assert.NoError(t, err)

	// Make sure the JAR will be downloaded from ojo
	err = os.Unsetenv(utils.ExtractorsRemoteEnv)
	assert.NoError(t, err)

	// Download JAR
	dependenciesPath, gradlePluginFilename, err := downloadGradleDependencies()
	assert.NoError(t, err)

	// Make sure the Gradle build-info extractor JAR exist
	expectedJarPath := filepath.Join(dependenciesPath, gradlePluginFilename)
	assert.FileExists(t, expectedJarPath)
}
