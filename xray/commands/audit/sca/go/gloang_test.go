package _go

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"os"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/stretchr/testify/assert"
)

func TestBuildGoDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := sca.CreateTestWorkspace(t, "go-project")
	defer cleanUp()

	err := removeTxtSuffix("go.mod.txt")
	assert.NoError(t, err)
	err = removeTxtSuffix("go.sum.txt")
	assert.NoError(t, err)
	err = removeTxtSuffix("test.go.txt")
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	server := &config.ServerDetails{
		Url:            "https://api.go.here",
		ArtifactoryUrl: "https://api.go.here/artifactory",
		User:           "user",
		AccessToken:    "sdsdccs2232",
	}
	goVersionID, err := getGoVersionAsDependency()
	assert.NoError(t, err)
	expectedUniqueDeps := []string{
		goPackageTypeIdentifier + "golang.org/x/text:v0.3.3",
		goPackageTypeIdentifier + "rsc.io/quote:v1.5.2",
		goPackageTypeIdentifier + "rsc.io/sampler:v1.3.0",
		goPackageTypeIdentifier + "testGoList",
		goVersionID.Id,
	}

	auditBasicParams := (&xrayutils.AuditBasicParams{}).SetServerDetails(server).SetDepsRepo("test-remote")
	rootNode, uniqueDeps, err := BuildDependencyTree(auditBasicParams)
	assert.NoError(t, err)
	assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")

	assert.Equal(t, "https://user:sdsdccs2232@api.go.here/artifactoryapi/go/test-remote|direct", os.Getenv("GOPROXY"))
	assert.NotEmpty(t, rootNode)

	// Check root module
	assert.Equal(t, rootNode[0].Id, goPackageTypeIdentifier+"testGoList")
	assert.Len(t, rootNode[0].Nodes, 3)

	// Test go version node
	goVersion, err := utils.GetParsedGoVersion()
	assert.NoError(t, err)
	sca.GetAndAssertNode(t, rootNode[0].Nodes, strings.ReplaceAll(goVersion.GetVersion(), "go", goSourceCodePrefix))

	// Test child without sub nodes
	child1 := sca.GetAndAssertNode(t, rootNode[0].Nodes, "golang.org/x/text:v0.3.3")
	assert.Len(t, child1.Nodes, 0)

	// Test child with 1 sub node
	child2 := sca.GetAndAssertNode(t, rootNode[0].Nodes, "rsc.io/quote:v1.5.2")
	assert.Len(t, child2.Nodes, 1)
	sca.GetAndAssertNode(t, child2.Nodes, "rsc.io/sampler:v1.3.0")
}

func removeTxtSuffix(txtFileName string) error {
	// go.sum.txt  >> go.sum
	return fileutils.MoveFile(txtFileName, strings.TrimSuffix(txtFileName, ".txt"))
}
