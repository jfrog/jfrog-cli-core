package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/stretchr/testify/assert"
)

var moduleNameTestProvider = []struct {
	projectName         string
	moduleName          string
	expectedModuleName  string
	expectedPackageName string
}{
	{"setuppyproject", "", "jfrog-python-example", "jfrog-python-example"},
	{"setuppyproject", "overidden-module", "overidden-module", "jfrog-python-example"},
	{"requirementsproject", "", "", ""},
	{"requirementsproject", "overidden-module", "overidden-module", ""},
}

func TestDetermineModuleName(t *testing.T) {
	pythonExecutablePath, err := getExecutablePath("python")
	assert.NoError(t, err)

	for _, test := range moduleNameTestProvider {
		t.Run(strings.Join([]string{test.projectName, test.moduleName}, "/"), func(t *testing.T) {
			// Prepare test
			command := &PythonCommand{buildConfiguration: utils.NewBuildConfiguration("", "", test.moduleName, "")}
			restoreCwd := changeToProjectDir(t, test.projectName)

			// Determine module name
			err, packageName := command.determineModuleName(pythonExecutablePath)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedModuleName, command.buildConfiguration.GetModule())
			assert.Equal(t, test.expectedPackageName, packageName)

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
