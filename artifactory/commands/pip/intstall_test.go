package pip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	piputils "github.com/jfrog/jfrog-cli-core/artifactory/utils/pip"
	"github.com/stretchr/testify/assert"
)

var moduleNameTestProvider = []struct {
	projectName        string
	moduleName         string
	expectedModuleName string
}{
	{"setuppyproject", "", "jfrog-python-example"},
	{"setuppyproject", "overidden-module", "overidden-module"},
	{"requirementsproject", "", ""},
	{"requirementsproject", "overidden-module", "overidden-module"},
}

func TestDetermineModuleName(t *testing.T) {
	command := NewPipInstallCommand()
	pythonExecutablePath, err := piputils.GetExecutablePath("python")
	assert.NoError(t, err)

	for _, test := range moduleNameTestProvider {
		t.Run(strings.Join([]string{test.projectName, test.moduleName}, "/"), func(t *testing.T) {
			// Prepare test
			command.buildConfiguration = &utils.BuildConfiguration{Module: test.moduleName}
			restoreCwd := changeToProjectDir(t, test.projectName)

			// Determine module name
			err = command.determineModuleName(pythonExecutablePath)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedModuleName, command.buildConfiguration.Module)

			// Cleanup
			restoreCwd()
		})
	}
}

func changeToProjectDir(t *testing.T, projectName string) func() {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	testdataDir := filepath.Join("..", "testdata", "pip", projectName)
	assert.NoError(t, os.Chdir(testdataDir))
	return func() {
		assert.NoError(t, os.Chdir(cwd))
	}
}
