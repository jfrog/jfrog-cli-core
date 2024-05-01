package xray

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
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

type DepTreeNode struct {
	Classifier *string   `json:"classifier"`
	Types      *[]string `json:"types"`
	Children   []string  `json:"children"`
}

func toNodeTypesMap(depMap map[string]DepTreeNode) map[string]*DepTreeNode {
	mapOfTypes := map[string]*DepTreeNode{}
	for nodId, value := range depMap {
		mapOfTypes[nodId] = nil
		if value.Types != nil || value.Classifier != nil {
			mapOfTypes[nodId] = &DepTreeNode{
				Classifier: value.Classifier,
				Types:      value.Types,
			}
		}
	}
	return mapOfTypes
}

func BuildXrayDependencyTree(treeHelper map[string]DepTreeNode, nodeId string) (*xrayUtils.GraphNode, map[string]*DepTreeNode) {
	rootNode := &xrayUtils.GraphNode{
		Id:    nodeId,
		Nodes: []*xrayUtils.GraphNode{},
	}
	dependencyAppearances := map[string]int8{}
	populateXrayDependencyTree(rootNode, treeHelper, dependencyAppearances)
	return rootNode, toNodeTypesMap(treeHelper)
}

func populateXrayDependencyTree(currNode *xrayUtils.GraphNode, treeHelper map[string]DepTreeNode, dependencyAppearances map[string]int8) {
	dependencyAppearances[currNode.Id]++
	if _, ok := treeHelper[currNode.Id]; !ok {
		treeHelper[currNode.Id] = DepTreeNode{}
	}
	// Recursively create & append all node's dependencies.
	for _, childDepId := range treeHelper[currNode.Id].Children {
		childNode := &xrayUtils.GraphNode{
			Id:         childDepId,
			Nodes:      []*xrayUtils.GraphNode{},
			Parent:     currNode,
			Types:      treeHelper[childDepId].Types,
			Classifier: treeHelper[childDepId].Classifier,
		}
		if dependencyAppearances[childDepId] >= maxUniqueAppearances || childNode.NodeHasLoop() {
			continue
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateXrayDependencyTree(childNode, treeHelper, dependencyAppearances)
	}
}
