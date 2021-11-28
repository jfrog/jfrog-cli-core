package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type AuditCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           xraycommands.OutputFormat
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

func (auditCmd *AuditCommand) SetOutputFormat(format xraycommands.OutputFormat) *AuditCommand {
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
		ScanType:   services.Dependency,
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
	return xrutils.PrintScanResults(results, auditCmd.outputFormat == xraycommands.Table, auditCmd.includeVulnerabilities, auditCmd.includeLicenses, false)
}

func CreateTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	err = fileutils.CopyDir(filepath.Join("..", "..", "testdata", sourceDir), tempDirPath, true, nil)
	assert.NoError(t, err)
	err = os.Chdir(tempDirPath)
	assert.NoError(t, err)
	return tempDirPath, func() {
		assert.NoError(t, os.Chdir(cwd))
		//assert.NoError(t, fileutils.RemoveTempDir(tempDirPath))
	}
}

func GetAndAssertNode(t *testing.T, modules []*services.GraphNode, moduleId string) *services.GraphNode {
	module := GetModule(modules, moduleId)
	assert.NotNil(t, module, "Module '"+moduleId+"' doesn't exist")
	return module
}

func GetModule(modules []*services.GraphNode, moduleId string) *services.GraphNode {
	for _, module := range modules {
		splitIdentifier := strings.Split(module.Id, "//")
		id := splitIdentifier[0]
		if len(splitIdentifier) > 1 {
			id = splitIdentifier[1]
		}
		if id == moduleId {
			return module
		}
	}
	return nil
}
