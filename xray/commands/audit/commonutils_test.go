package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
)

func createTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	err = fileutils.CopyDir(filepath.Join("..", "testdata", sourceDir), tempDirPath, true, nil)
	assert.NoError(t, err)
	err = os.Chdir(tempDirPath)
	assert.NoError(t, err)
	return tempDirPath, func() {
		assert.NoError(t, os.Chdir(cwd))
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath))
	}
}

func getAndAssertNode(t *testing.T, modules []*services.GraphNode, moduleId string) *services.GraphNode {
	module := getModule(modules, moduleId)
	assert.NotNil(t, module, "Module '"+moduleId+"' doesn't exist")
	return module
}

func getModule(modules []*services.GraphNode, moduleId string) *services.GraphNode {
	for _, module := range modules {
		splitIdentifier := strings.Split(module.Id, "//")
		id := splitIdentifier[0]
		if len(splitIdentifier) > 1 {
			id = splitIdentifier[1]
		}
		if id == moduleId {
			return module
		}
	}
	return nil
}
