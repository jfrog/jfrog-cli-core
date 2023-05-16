package yarn

import (
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"testing"

	biutils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
)

func TestParseYarnDependenciesList(t *testing.T) {
	yarnDependencies := map[string]*biutils.YarnDependency{
		"pack1@npm:1.0.0": {Value: "pack1@npm:1.0.0", Details: biutils.YarnDepDetails{Version: "1.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack4@npm:4.0.0"}}}},
		"pack2@npm:2.0.0": {Value: "pack2@npm:2.0.0", Details: biutils.YarnDepDetails{Version: "2.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack4@npm:4.0.0"}, {Locator: "pack5@npm:5.0.0"}}}},
		"pack3@npm:3.0.0": {Value: "pack3@npm:3.0.0", Details: biutils.YarnDepDetails{Version: "3.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack1@virtual:c192f6b3b32cd5d11a443144e162ec3bc#npm:1.0.0"}, {Locator: "pack2@npm:2.0.0"}}}},
		"pack4@npm:4.0.0": {Value: "pack4@npm:4.0.0", Details: biutils.YarnDepDetails{Version: "4.0.0"}},
		"pack5@npm:5.0.0": {Value: "pack5@npm:5.0.0", Details: biutils.YarnDepDetails{Version: "5.0.0", Dependencies: []biutils.YarnDependencyPointer{{Locator: "pack2@npm:2.0.0"}}}},
	}

	packageInfo := &biutils.PackageInfo{Name: "pack3", Version: "3.0.0"}
	expectedTree := &xrayUtils.GraphNode{
		Id: "npm://pack3:3.0.0",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "npm://pack1:1.0.0",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://pack4:4.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
				}},
			{Id: "npm://pack2:2.0.0",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://pack4:4.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
					{Id: "npm://pack5:5.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
				}},
		},
	}

	xrayDependenciesTree := parseYarnDependenciesMap(yarnDependencies, packageInfo)

	equals := tests.CompareTree(expectedTree, xrayDependenciesTree)
	if !equals {
		t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
	}
}
