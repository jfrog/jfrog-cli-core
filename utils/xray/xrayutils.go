package xray

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/exp/maps"
)

func CreateXrayServiceManager(serviceDetails *config.ServerDetails) (*xray.XrayServicesManager, error) {
	xrayDetails, err := serviceDetails.CreateXrayAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientconfig.NewConfigBuilder().
		SetServiceDetails(xrayDetails).
		Build()
	if err != nil {
		return nil, err
	}
	return xray.New(serviceConfig)
}

func CreateXrayServiceManagerAndGetVersion(serviceDetails *config.ServerDetails) (*xray.XrayServicesManager, string, error) {
	xrayManager, err := CreateXrayServiceManager(serviceDetails)
	if err != nil {
		return nil, "", err
	}
	xrayVersion, err := xrayManager.GetVersion()
	if err != nil {
		return nil, "", err
	}
	return xrayManager, xrayVersion, nil
}

const maxUniqueAppearances = 10

func BuildXrayDependencyTree(treeHelper map[string][]string, nodeId string) (*xrayUtils.GraphNode, []string) {
	rootNode := &xrayUtils.GraphNode{
		Id:    nodeId,
		Nodes: []*xrayUtils.GraphNode{},
	}
	dependencyAppearances := map[string]int8{}
	populateXrayDependencyTree(rootNode, treeHelper, &dependencyAppearances)
	return rootNode, maps.Keys(dependencyAppearances)
}

func populateXrayDependencyTree(currNode *xrayUtils.GraphNode, treeHelper map[string][]string, dependencyAppearances *map[string]int8) {
	(*dependencyAppearances)[currNode.Id]++
	// Recursively create & append all node's dependencies.
	for _, childDepId := range treeHelper[currNode.Id] {
		childNode := &xrayUtils.GraphNode{
			Id:     childDepId,
			Nodes:  []*xrayUtils.GraphNode{},
			Parent: currNode,
		}
		if (*dependencyAppearances)[childDepId] >= maxUniqueAppearances || childNode.NodeHasLoop() {
			continue
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateXrayDependencyTree(childNode, treeHelper, dependencyAppearances)
	}
}
