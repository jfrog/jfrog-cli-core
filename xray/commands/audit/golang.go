package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

const (
	goPackageTypeIdentifier = "go://"
)

type AuditGoCommand struct {
	AuditCommand
}

func NewEmptyAuditGoCommand() *AuditGoCommand {
	return &AuditGoCommand{AuditCommand: *NewAuditCommand()}
}

func NewAuditGoCommand(auditCmd AuditCommand) *AuditGoCommand {
	return &AuditGoCommand{AuditCommand: auditCmd}
}

func (auditCmd *AuditGoCommand) Run() (err error) {
	rootNode, err := auditCmd.buildGoDependencyTree()
	if err != nil {
		return err
	}
	return auditCmd.ScanDependencyTree([]*services.GraphNode{rootNode})
}

func (auditCmd *AuditGoCommand) buildGoDependencyTree() (*services.GraphNode, error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return nil, err
	}
	// Calculate go dependencies graph
	dependenciesGraph, err := goutils.GetDependenciesGraph(currentDir)
	if err != nil {
		return nil, err
	}
	// Calculate go dependencies list
	dependenciesList, err := goutils.GetDependenciesList(currentDir)
	if err != nil {
		return nil, err
	}
	// Get root module name
	rootModuleName, err := goutils.GetModuleName(currentDir)
	if err != nil {
		return nil, err
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := &services.GraphNode{
		Id:    goPackageTypeIdentifier + rootModuleName,
		Nodes: []*services.GraphNode{},
	}
	populateGoDependencyTree(rootNode, dependenciesGraph, dependenciesList)
	return rootNode, err
}

func populateGoDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string, dependenciesList map[string]bool) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, goPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, childName := range currDepChildren {
		if dependenciesList[strings.ReplaceAll(childName, ":", "@v")] == false {
			// 'go list all' is more accurate than 'go graph' so we filter out deps that don't exist in go list
			continue
		}
		childNode := &services.GraphNode{
			Id:     goPackageTypeIdentifier + childName,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateGoDependencyTree(childNode, dependenciesGraph, dependenciesList)
	}
}

func (na *AuditGoCommand) CommandName() string {
	return "xr_audit_go"
}
