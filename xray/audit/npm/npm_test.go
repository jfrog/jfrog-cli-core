package npm

import (
	"encoding/json"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"testing"

	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/stretchr/testify/assert"
)

func TestParseNpmDependenciesList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "npm")
	defer cleanUp()
	dependenciesJson, err := os.ReadFile("dependencies.json")
	assert.NoError(t, err)
	var dependencies []buildinfo.Dependency
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)
	packageInfo := &biutils.PackageInfo{Name: "npmexmaple", Version: "0.1.0"}
	looseEnvifyJsTokens := []*xrayUtils.GraphNode{{Id: "npm://loose-envify:1.4.0", Nodes: []*xrayUtils.GraphNode{{Id: "npm://js-tokens:4.0.0"}}}}
	expectedTree := &xrayUtils.GraphNode{
		Id: "npm://npmexmaple:0.1.0",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "npm://next-auth:4.22.1",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://react-dom:18.2.0", Nodes: []*xrayUtils.GraphNode{
						{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
						{Id: "npm://loose-envify:1.4.0", Nodes: []*xrayUtils.GraphNode{{Id: "npm://js-tokens:4.0.0"}}},
						{Id: "npm://scheduler:0.23.0", Nodes: looseEnvifyJsTokens},
					}},
					{Id: "npm://jose:4.14.4", Nodes: []*xrayUtils.GraphNode{}},
					{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
					{Id: "npm://uuid:8.3.2", Nodes: []*xrayUtils.GraphNode{}},
					{Id: "npm://openid-client:5.4.2", Nodes: []*xrayUtils.GraphNode{
						{Id: "npm://jose:4.14.4"},
						{Id: "npm://lru-cache:6.0.0", Nodes: []*xrayUtils.GraphNode{{Id: "npm://yallist:4.0.0"}}},
						{Id: "npm://oidc-token-hash:5.0.3", Nodes: []*xrayUtils.GraphNode{}},
						{Id: "npm://object-hash:2.2.0", Nodes: []*xrayUtils.GraphNode{}},
					}},
					{Id: "npm://next:12.0.10", Nodes: []*xrayUtils.GraphNode{
						{Id: "npm://react-dom:18.2.0", Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
							{Id: "npm://loose-envify:1.4.0", Nodes: []*xrayUtils.GraphNode{{Id: "npm://js-tokens:4.0.0"}}},
							{Id: "npm://scheduler:0.23.0", Nodes: looseEnvifyJsTokens}}},
						{Id: "npm://styled-jsx:5.0.0"},
						{Id: "npm://@next/swc-darwin-arm64:12.0.10"},
						{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
						{Id: "npm://@next/env:12.0.10"},
						{Id: "npm://caniuse-lite:1.0.30001486"},
						{Id: "npm://postcss:8.4.5", Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://picocolors:1.0.0"},
							{Id: "npm://source-map-js:1.0.2"},
							{Id: "npm://nanoid:3.3.6"},
						}},
						{Id: "npm://use-subscription:1.5.1", Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://object-assign:4.1.1"},
						}},
					}},
					{Id: "npm://@panva/hkdf:1.1.1"},
					{Id: "npm://preact-render-to-string:5.2.6", Nodes: []*xrayUtils.GraphNode{
						{Id: "npm://pretty-format:3.8.0"},
						{Id: "npm://preact:10.13.2"},
					}},
					{Id: "npm://preact:10.13.2"},
					{Id: "npm://@babel/runtime:7.21.5", Nodes: []*xrayUtils.GraphNode{
						{Id: "npm://regenerator-runtime:0.13.11"},
					}},
					{Id: "npm://cookie:0.5.0"},
					{Id: "npm://oauth:0.9.15"},
				}},
			{Id: "npm://next:12.0.10", Nodes: []*xrayUtils.GraphNode{
				{Id: "npm://react-dom:18.2.0", Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
					{Id: "npm://loose-envify:1.4.0", Nodes: []*xrayUtils.GraphNode{{Id: "npm://js-tokens:4.0.0"}}},
					{Id: "npm://scheduler:0.23.0", Nodes: looseEnvifyJsTokens}}},
				{Id: "npm://styled-jsx:5.0.0"},
				{Id: "npm://@next/swc-darwin-arm64:12.0.10"},
				{Id: "npm://react:18.2.0", Nodes: looseEnvifyJsTokens},
				{Id: "npm://@next/env:12.0.10"},
				{Id: "npm://caniuse-lite:1.0.30001486"},
				{Id: "npm://postcss:8.4.5", Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://picocolors:1.0.0"},
					{Id: "npm://source-map-js:1.0.2"},
					{Id: "npm://nanoid:3.3.6"},
				}},
				{Id: "npm://use-subscription:1.5.1", Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://object-assign:4.1.1"},
				}},
			}},
		},
	}

	xrayDependenciesTree := parseNpmDependenciesList(dependencies, packageInfo)
	equals := tests.CompareTree(expectedTree, xrayDependenciesTree)
	if !equals {
		t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
	}

}

func TestIgnoreScripts(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "npm-scripts")
	defer cleanUp()

	// The package.json file contain a postinstall script running an "exit 1" command.
	// Without the "--ignore-scripts" flag, the test will fail.
	_, err := BuildDependencyTree([]string{})
	assert.NoError(t, err)
}
