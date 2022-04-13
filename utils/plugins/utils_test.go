package plugins

import (
	"fmt"
	configtests "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

const pluginName = "rt-fs"

func TestConvertPluginsV0ToV1(t *testing.T) {
	// Setup testing env
	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	testHomeDir := setupPluginsTestingEnv(t, "v0")
	// Migration- v0 to v1
	p, err := convertPluginsV0ToV1()
	assert.NoError(t, err)
	assert.Equal(t, p.Version, 1)
	// Verity plugins.yaml exists
	expectedYamlLocation := filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, coreutils.JfrogPluginsFile)
	exists, err := fileutils.IsFileExists(expectedYamlLocation, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("expected file: %s doesn't exists", expectedYamlLocation))
	// Verify executable location according to v1 hierarchy
	exists, err = fileutils.IsFileExists(filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, pluginName, coreutils.PluginsExecDirName, GetLocalPluginExecutableName(pluginName)), false)
	assert.NoError(t, err)
	assert.True(t, exists)
}

// Plugins directory is empty - only 'plugins.yaml' should be created.
func TestConvertPluginsV0ToV1EmptyDir(t *testing.T) {
	// Setup testing env
	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	testHomeDir := setupPluginsTestingEnv(t, "empty")
	// Migration- v0 to v1
	p, err := convertPluginsV0ToV1()
	assert.NoError(t, err)
	assert.Equal(t, p.Version, 1)
	// Verity plugins.yaml exists
	expectedYamlLocation := filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, coreutils.JfrogPluginsFile)
	exists, err := fileutils.IsFileExists(expectedYamlLocation, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("expected file: %s doesn't exists", expectedYamlLocation))
}

// Plugins directory contains unexpected file (non executable)
func TestConvertPluginsV0ToV1WithUnexpectedFiles(t *testing.T) {
	// Setup testing env
	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	testHomeDir := setupPluginsTestingEnv(t, "unexpectedFiles")
	// Migration- v0 to v1
	p, err := convertPluginsV0ToV1()
	assert.NoError(t, err)
	assert.Equal(t, p.Version, 1)
	// Verity plugins.yaml exists
	expectedYamlLocation := filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, coreutils.JfrogPluginsFile)
	exists, err := fileutils.IsFileExists(expectedYamlLocation, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("expected file: %s doesn't exists", expectedYamlLocation))
	// Verify executable location according to v1 hierarchy
	exists, err = fileutils.IsFileExists(filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, pluginName, coreutils.PluginsExecDirName, GetLocalPluginExecutableName(pluginName)), false)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func setupPluginsTestingEnv(t *testing.T, pluginsDirName string) string {
	testHomeDir, err := coreutils.GetJfrogHomeDir()
	assert.NoError(t, err)
	wd, err := os.Getwd()
	assert.NoError(t, err)
	err = fileutils.CopyDir(filepath.Join(wd, "testdata", coreutils.JfrogPluginsDirName), testHomeDir, true, nil)
	assert.NoError(t, err)
	err = fileutils.RenamePath(filepath.Join(testHomeDir, pluginsDirName), filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName))
	assert.NoError(t, err)
	pluginsExecName := filepath.Join(testHomeDir, coreutils.JfrogPluginsDirName, GetLocalPluginExecutableName(pluginName))
	exists, err := fileutils.IsFileExists(pluginsExecName, false)
	if exists {
		err = os.Chmod(pluginsExecName, 0777)
		assert.NoError(t, err)
	}
	return testHomeDir
}
