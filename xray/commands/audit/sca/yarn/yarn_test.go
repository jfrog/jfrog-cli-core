package yarn

import (
	"github.com/jfrog/build-info-go/build"
	biutils "github.com/jfrog/build-info-go/build/utils"
	utils2 "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestParseYarnDependenciesList(t *testing.T) {
	yarnDependencies := map[string]*biutils.YarnDependency{
		"pack1@npm:1.0.0":        {Value: "pack1@npm:1.0.0", Details: biutils.YarnDepDetails{Version: "1.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack4@npm:4.0.0"}}}},
		"pack2@npm:2.0.0":        {Value: "pack2@npm:2.0.0", Details: biutils.YarnDepDetails{Version: "2.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack4@npm:4.0.0"}, {Locator: "pack5@npm:5.0.0"}}}},
		"@jfrog/pack3@npm:3.0.0": {Value: "@jfrog/pack3@npm:3.0.0", Details: biutils.YarnDepDetails{Version: "3.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack1@virtual:c192f6b3b32cd5d11a443144e162ec3bc#npm:1.0.0"}, {Locator: "pack2@npm:2.0.0"}}}},
		"pack4@npm:4.0.0":        {Value: "pack4@npm:4.0.0", Details: biutils.YarnDepDetails{Version: "4.0.0"}},
		"pack5@npm:5.0.0":        {Value: "pack5@npm:5.0.0", Details: biutils.YarnDepDetails{Version: "5.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack2@npm:2.0.0"}}}},
	}

	rootXrayId := utils.NpmPackageTypeIdentifier + "@jfrog/pack3:3.0.0"
	expectedTree := &xrayUtils.GraphNode{
		Id: rootXrayId,
		Nodes: []*xrayUtils.GraphNode{
			{Id: utils.NpmPackageTypeIdentifier + "pack1:1.0.0",
				Nodes: []*xrayUtils.GraphNode{
					{Id: utils.NpmPackageTypeIdentifier + "pack4:4.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
				}},
			{Id: utils.NpmPackageTypeIdentifier + "pack2:2.0.0",
				Nodes: []*xrayUtils.GraphNode{
					{Id: utils.NpmPackageTypeIdentifier + "pack4:4.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
					{Id: utils.NpmPackageTypeIdentifier + "pack5:5.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
				}},
		},
	}
	expectedUniqueDeps := []string{
		utils.NpmPackageTypeIdentifier + "pack1:1.0.0",
		utils.NpmPackageTypeIdentifier + "pack2:2.0.0",
		utils.NpmPackageTypeIdentifier + "pack4:4.0.0",
		utils.NpmPackageTypeIdentifier + "pack5:5.0.0",
		utils.NpmPackageTypeIdentifier + "@jfrog/pack3:3.0.0",
	}

	xrayDependenciesTree, uniqueDeps := parseYarnDependenciesMap(yarnDependencies, rootXrayId)
	assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
	assert.True(t, tests.CompareTree(expectedTree, xrayDependenciesTree), "expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
}

func TestIsYarnProjectInstalled(t *testing.T) {
	tempDirPath, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	yarnProjectPath := filepath.Join("..", "..", "..", "testdata", "yarn-project")
	assert.NoError(t, utils2.CopyDir(yarnProjectPath, tempDirPath, true, nil))
	projectInstalled, err := isYarnProjectInstalled(tempDirPath)
	assert.NoError(t, err)
	assert.False(t, projectInstalled)
	executablePath, err := biutils.GetYarnExecutable()
	assert.NoError(t, err)

	// We install the project and check again to verify we get the correct answer
	assert.NoError(t, build.RunYarnCommand(executablePath, tempDirPath, "install"))
	projectInstalled, err = isYarnProjectInstalled(tempDirPath)
	assert.NoError(t, err)
	assert.True(t, projectInstalled)
}

func TestRunYarnInstallAccordingToVersion(t *testing.T) {
	executeRunYarnInstallAccordingToVersionAndVerifyInstallation(t, []string{})
	executeRunYarnInstallAccordingToVersionAndVerifyInstallation(t, []string{"install", "--mode=update-lockfile"})
}

func executeRunYarnInstallAccordingToVersionAndVerifyInstallation(t *testing.T, params []string) {
	tempDirPath, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	yarnProjectPath := filepath.Join("..", "..", "..", "testdata", "yarn-project")
	assert.NoError(t, utils2.CopyDir(yarnProjectPath, tempDirPath, true, nil))

	executablePath, err := biutils.GetYarnExecutable()
	assert.NoError(t, err)

	err = runYarnInstallAccordingToVersion(tempDirPath, executablePath, params)
	assert.NoError(t, err)

	// Checking the installation worked
	projectInstalled, err := isYarnProjectInstalled(tempDirPath)
	assert.NoError(t, err)
	assert.True(t, projectInstalled)
}
