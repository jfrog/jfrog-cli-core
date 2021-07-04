package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	NpmPackageTypeIdentifier = "npm://"
)

type AuditNpmCommand struct {
	serverDetails          *config.ServerDetails
	workingDirectory       string
	arguments              []string
	typeRestriction        npmutils.TypeRestriction
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLincenses       bool
}

func (auditCmd *AuditNpmCommand) SetWorkingDirectory(dir string) *AuditNpmCommand {
	auditCmd.workingDirectory = dir
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetArguments(args []string) *AuditNpmCommand {
	auditCmd.arguments = args
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetNpmTypeRestriction(typeRestriction npmutils.TypeRestriction) *AuditNpmCommand {
	auditCmd.typeRestriction = typeRestriction
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetServerDetails(server *config.ServerDetails) *AuditNpmCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditNpmCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditNpmCommand) SetWatches(watches []string) *AuditNpmCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetProject(project string) *AuditNpmCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetTargetRepoPath(repoPath string) *AuditNpmCommand {
	auditCmd.projectKey = repoPath
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetIncludeVulnerabilities(include bool) *AuditNpmCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditNpmCommand) SetIncludeLincenses(include bool) *AuditNpmCommand {
	auditCmd.includeLincenses = include
	return auditCmd
}

func NewAuditNpmCommand() *AuditNpmCommand {
	return &AuditNpmCommand{}
}

func (auditCmd *AuditNpmCommand) Run() (err error) {
	typeRestriction := auditCmd.typeRestriction

	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	if auditCmd.workingDirectory == "" {
		auditCmd.workingDirectory = currentDir
	} else {
		err = os.Chdir(auditCmd.workingDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = os.Chdir(currentDir)
		}()
	}
	log.Debug("Working directory set to:", auditCmd.workingDirectory)

	packageInfo, err := coreutils.ReadPackageInfoFromPackageJson(auditCmd.workingDirectory)
	if err != nil {
		return err
	}
	npmExecutablePath, err := npmutils.FindNpmExecutable()
	if err != nil {
		return err
	}
	// Calculate npm dependencies
	dependenciesList, err := npmutils.CalculateDependenciesList(typeRestriction, []string{}, npmExecutablePath, packageInfo.BuildInfoModuleId())
	if err != nil {
		return err
	}
	// Parse the dependencies into an Xray dependency tree format
	npmGraph := parseNpmDependenciesList(dependenciesList, packageInfo)
	xrayManager, err := commands.CreateXrayServiceManager(auditCmd.serverDetails)
	if err != nil {
		return err
	}
	params := services.NewXrayGraphScanParams()
	params.Graph = npmGraph
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
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return err
	}
	log.Info("The full scan results are available here: " + tempDirPath)
	if err = xrutils.WriteJsonResults(scanResults, tempDirPath); err != nil {
		return err
	}
	if len(scanResults.Violations) > 0 {
		err = xrutils.PrintViolationsTable(scanResults.Violations)
	}
	if len(scanResults.Vulnerabilities) > 0 {
		xrutils.PrintVulnerabilitiesTable(scanResults.Vulnerabilities)
	}
	return err

}

func parseNpmDependenciesList(dependencies map[string]*npmutils.Dependency, packageInfo *coreutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for dependencyId, dependency := range dependencies {
		dependencyId = NpmPackageTypeIdentifier + dependencyId
		parent := NpmPackageTypeIdentifier + dependency.GetPathToRoot()[0][0]
		if children, ok := treeMap[parent]; ok {
			treeMap[parent] = append(children, dependencyId)
		} else {
			treeMap[parent] = []string{dependencyId}
		}
	}
	return buildXrayDependencyTree(treeMap, NpmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}

func buildXrayDependencyTree(treeHelper map[string][]string, nodeId string) *services.GraphNode {
	// Initialize the new node
	xrDependencyTree := &services.GraphNode{}
	xrDependencyTree.Id = nodeId
	xrDependencyTree.Nodes = []*services.GraphNode{}
	// Recursively create & append all node's dependencies.
	for _, dependency := range treeHelper[nodeId] {
		xrDependencyTree.Nodes = append(xrDependencyTree.Nodes, buildXrayDependencyTree(treeHelper, dependency))

	}
	return xrDependencyTree
}

func (auditCmd *AuditNpmCommand) CommandName() string {
	return "xr_audit_npm"
}
