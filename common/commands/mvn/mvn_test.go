package mvn

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
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

	// Download JAR and create classworlds.conf
	dependenciesPath, err := downloadDependencies()
	assert.NoError(t, err)

	// Make sure the Maven build-info extractor JAR and the classwords.conf file exists
	expectedJarPath := filepath.Join(dependenciesPath, fmt.Sprintf("build-info-extractor-maven3-%s-uber.jar", mavenExtractorDependencyVersion))
	assert.FileExists(t, expectedJarPath)
	expectedClasswordsPath := filepath.Join(dependenciesPath, "classworlds.conf")
	assert.FileExists(t, expectedClasswordsPath)
}
