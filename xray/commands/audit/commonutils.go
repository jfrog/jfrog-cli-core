package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

type AuditCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           xrutils.OutputFormat
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

func (auditCmd *AuditCommand) SetOutputFormat(format xrutils.OutputFormat) *AuditCommand {
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

func (auditCmd *AuditCommand) ScanDependencyTree(modulesDependencyTrees []*services.GraphNode) error {
	var results []services.ScanResponse
	params := services.XrayGraphScanParams{
		RepoPath:   auditCmd.targetRepoPath,
		Watches:    auditCmd.watches,
		ProjectKey: auditCmd.projectKey,
	}
	for _, moduleDependencyTree := range modulesDependencyTrees {
		params.Graph = moduleDependencyTree
		// Log the scanned module ID
		moduleName := moduleDependencyTree.Id[strings.Index(moduleDependencyTree.Id, "//")+2:]
		log.Info("Scanning module " + moduleName + "...")

		scanResults, err := xraycommands.RunScanGraphAndGetResults(auditCmd.serverDetails, params, auditCmd.includeVulnerabilities, auditCmd.includeLicenses)
		if err != nil {
			log.Error(fmt.Sprintf("Scanning %s failed with error: %s", moduleName, err.Error()))
			break
		}
		results = append(results, *scanResults)
	}
	if results == nil || len(results) < 1 {
		// if all scans failed, fail the audit command
		return errors.New("audit command failed due to Xray internal error")
	}
	err := xrutils.PrintScanResults(results, auditCmd.outputFormat == xrutils.Table, auditCmd.includeVulnerabilities, auditCmd.includeLicenses, false)
	if err != nil {
		return err
	}
	// If includeVulnerabilities is false it means that context was provided, so we need to check for build violations
	if auditCmd.includeVulnerabilities == false {
		if xrutils.CheckIfFailBuild(results) {
			return xrutils.NewFailBuildError()
		}
	}

	return nil
}
