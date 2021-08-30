package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
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
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}

	// Calculate npm dependencies
	dependenciesMap, err := goutils.GetDependenciesGraph(currentDir)
	if err != nil {
		return err
	}

	// Calculate npm dependencies
	dependenciesList, err := goutils.GetDependenciesList(currentDir)
	if err != nil {
		return err
	}
	// Get Root module name
	rootModuleName, err := goutils.GetModuleName(currentDir)
	if err != nil {
		return err
	}
	// Parse the dependencies into Xray dependency tree format
	goGraph := &services.GraphNode{
		Id:    goPackageTypeIdentifier + rootModuleName,
		Nodes: []*services.GraphNode{},
	}
	buildGoDependencyTree(goGraph, dependenciesMap, dependenciesList)

	xrayManager, err := commands.CreateXrayServiceManager(auditCmd.serverDetails)
	if err != nil {
		return err
	}
	params := services.NewXrayGraphScanParams()
	params.Graph = goGraph
	params.RepoPath = auditCmd.targetRepoPath
	params.Watches = auditCmd.watches
	params.ProjectKey = auditCmd.projectKey

	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return err
	}
	scanResults, err := xrayManager.GetScanGraphResults(scanId, auditCmd.includeVulnerabilities, auditCmd.includeLincenses)
	if err != nil {
		return err
	}
	err = xrutils.PrintScanResults([]services.ScanResponse{*scanResults}, auditCmd.outputFormat == Table, auditCmd.includeVulnerabilities, auditCmd.includeLincenses, false)
	return err
}

func buildGoDependencyTree(currNode *services.GraphNode, dependenciesMap map[string][]string, dependenciesList map[string]bool) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesMap[strings.TrimPrefix(currNode.Id, goPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, childName := range currDepChildren {
		if dependenciesList[childName] == false {
			continue
		}
		childNode := &services.GraphNode{
			Id:     goPackageTypeIdentifier + childName,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		buildGoDependencyTree(childNode, dependenciesMap, dependenciesList)
	}
}

func (na *AuditGoCommand) CommandName() string {
	return "xr_audit_go"
}
