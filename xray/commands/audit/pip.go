package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	piputils "github.com/jfrog/jfrog-cli-core/v2/utils/pip"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

const (
	pipPackageTypeIdentifier = "pypi://"
)

type AuditPipCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           OutputFormat
	insecureTls            bool
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLincenses       bool
}

func (auditCmd *AuditPipCommand) SetServerDetails(server *config.ServerDetails) *AuditPipCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetOutputFormat(format OutputFormat) *AuditPipCommand {
	auditCmd.outputFormat = format
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetInsecureTls(insecureTls bool) *AuditPipCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *AuditPipCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditPipCommand) SetWatches(watches []string) *AuditPipCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetProject(project string) *AuditPipCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetTargetRepoPath(repoPath string) *AuditPipCommand {
	auditCmd.projectKey = repoPath
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetIncludeVulnerabilities(include bool) *AuditPipCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditPipCommand) SetIncludeLincenses(include bool) *AuditPipCommand {
	auditCmd.includeLincenses = include
	return auditCmd
}

func NewAuditPipCommand() *AuditPipCommand {
	return &AuditPipCommand{}
}

func (auditCmd *AuditPipCommand) Run() (err error) {
	rootNodes, err := auditCmd.buildPipDependencyTree()
	if err != nil {
		return err
	}
	err = RunScanGraph(rootNodes, auditCmd.serverDetails, auditCmd.includeVulnerabilities, auditCmd.includeLincenses, auditCmd.targetRepoPath, auditCmd.projectKey, auditCmd.watches, auditCmd.outputFormat)
	return err
}

func (auditCmd *AuditPipCommand) buildPipDependencyTree() ([]*services.GraphNode, error) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		e := fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()
	err = piputils.RunVirtualEnv(tempDirPath)
	if err != nil {
		return nil, err
	}

	// pip install project
	err = piputils.RunPipInstall(tempDirPath)
	if err != nil {
		return nil, err
	}

	// run pipdeptree.py to get dependencies tree
	dependenciesGraph2, parents, err := piputils.RunPipDepTree(tempDirPath)
	if err != nil {
		return nil, err
	}
	var modulesDependencyTrees []*services.GraphNode
	for _, parent := range parents {
		parentNode := &services.GraphNode{
			Id:    pipPackageTypeIdentifier + parent,
			Nodes: []*services.GraphNode{},
		}
		populatePipDependencyTree(parentNode, dependenciesGraph2)
		modulesDependencyTrees = append(modulesDependencyTrees, parentNode)
	}

	return modulesDependencyTrees, err
}

func populatePipDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, pipPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, dependency := range currDepChildren {
		childNode := &services.GraphNode{
			Id:     pipPackageTypeIdentifier + dependency,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populatePipDependencyTree(childNode, dependenciesGraph)
	}
}

func (auditCmd *AuditPipCommand) CommandName() string {
	return "xr_audit_pip"
}
