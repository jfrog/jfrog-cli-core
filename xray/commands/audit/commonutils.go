package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

type AuditCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           OutputFormat
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLicenses        bool
}

func NewAuditCommand() *AuditCommand {
	return &AuditCommand{}
}

func (auditCmd *AuditCommand) SetServerDetails(server *config.ServerDetails) *AuditCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditCommand) SetOutputFormat(format OutputFormat) *AuditCommand {
	auditCmd.outputFormat = format
	return auditCmd
}

func (auditCmd *AuditCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditCommand) SetWatches(watches []string) *AuditCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditCommand) SetProject(project string) *AuditCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditCommand) SetTargetRepoPath(repoPath string) *AuditCommand {
	auditCmd.targetRepoPath = repoPath
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeVulnerabilities(include bool) *AuditCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeLicenses(include bool) *AuditCommand {
	auditCmd.includeLicenses = include
	return auditCmd
}

func (auditCmd *AuditCommand) runScanGraph(modulesDependencyTrees []*services.GraphNode) error {
	xrayManager, err := commands.CreateXrayServiceManager(auditCmd.serverDetails)
	if err != nil {
		return err
	}
	var results []services.ScanResponse
	for _, moduleDependencyTree := range modulesDependencyTrees {
		params := &services.XrayGraphScanParams{
			Graph:      moduleDependencyTree,
			RepoPath:   auditCmd.targetRepoPath,
			Watches:    auditCmd.watches,
			ProjectKey: auditCmd.projectKey,
		}

		// Log the scanned module ID
		log.Info("Scanning module " + moduleDependencyTree.Id[strings.Index(moduleDependencyTree.Id, "//")+2:] + "...")

		// Scan and wait for results
		scanId, err := xrayManager.ScanGraph(*params)
		if err != nil {
			return err
		}
		scanResults, err := xrayManager.GetScanGraphResults(scanId, auditCmd.includeVulnerabilities, auditCmd.includeLicenses)
		if err != nil {
			return err
		}
		results = append(results, *scanResults)
	}
	err = xrutils.PrintScanResults(results, auditCmd.outputFormat == Table, auditCmd.includeVulnerabilities, auditCmd.includeLicenses, false)
	return err
}
