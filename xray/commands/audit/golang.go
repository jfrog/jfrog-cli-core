package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

const (
	goPackageTypeIdentifier = "go://"
)

type AuditGoCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           OutputFormat
	insecureTls            bool
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLincenses       bool
}

func (auditCmd *AuditGoCommand) SetServerDetails(server *config.ServerDetails) *AuditGoCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetOutputFormat(format OutputFormat) *AuditGoCommand {
	auditCmd.outputFormat = format
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetInsecureTls(insecureTls bool) *AuditGoCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *AuditGoCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditGoCommand) SetWatches(watches []string) *AuditGoCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetProject(project string) *AuditGoCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetTargetRepoPath(repoPath string) *AuditGoCommand {
	auditCmd.projectKey = repoPath
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetIncludeVulnerabilities(include bool) *AuditGoCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditGoCommand) SetIncludeLincenses(include bool) *AuditGoCommand {
	auditCmd.includeLincenses = include
	return auditCmd
}

func NewAuditGoCommand() *AuditGoCommand {
	return &AuditGoCommand{}
}

func (auditCmd *AuditGoCommand) Run() (err error) {
	rootNode, err := auditCmd.buildGoDependencyTree()
	if err != nil {
		return err
	}
	err = RunScanGraph([]*services.GraphNode{rootNode}, auditCmd.serverDetails, auditCmd.includeVulnerabilities, auditCmd.includeLincenses, auditCmd.targetRepoPath, auditCmd.projectKey, auditCmd.watches, auditCmd.outputFormat)
	return err
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
	// Get Root module name
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
			// 'go list all' is more accurate than 'go graph' so we filter out deps that doesn't exist in go list
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
