package audit

import (
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
)

func createTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	tempDirPath, createTempDirCallback := fileutils.CreateTempDirWithCallbackAndAssert(t)
	err := fileutils.CopyDir(filepath.Join("..", "testdata", sourceDir), tempDirPath, true, nil)
	assert.NoError(t, err)
	chdirCallback := testsutils.ChangeDirWithCallback(t, tempDirPath)
	return tempDirPath, func() {
		chdirCallback()
		createTempDirCallback()
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
