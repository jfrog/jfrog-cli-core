package audit

import (
	"encoding/json"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditMavenCommand struct {
	serverDetails          *config.ServerDetails
	insecureTls            bool
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLincenses       bool
}

func (auditCmd *AuditMavenCommand) SetServerDetails(server *config.ServerDetails) *AuditMavenCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetInsecureTls(insecureTls bool) *AuditMavenCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *AuditMavenCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditMavenCommand) SetWatches(watches []string) *AuditMavenCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetProject(project string) *AuditMavenCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetTargetRepoPath(repoPath string) *AuditMavenCommand {
	auditCmd.projectKey = repoPath
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetIncludeVulnerabilities(include bool) *AuditMavenCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetIncludeLincenses(include bool) *AuditMavenCommand {
	auditCmd.includeLincenses = include
	return auditCmd
}

func NewAuditMvnCommand() *AuditMavenCommand {
	return &AuditMavenCommand{}
}

func (auditCmd *AuditMavenCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	if err != nil {
		return
	}

	return runScanGraph(modulesDependencyTrees, auditCmd.serverDetails, auditCmd.includeVulnerabilities, auditCmd.includeLincenses, auditCmd.targetRepoPath, auditCmd.projectKey, auditCmd.watches)
}

func (auditCmd *AuditMavenCommand) getModulesDependencyTrees() (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = auditCmd.runMvn(buildConfiguration)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func (auditCmd *AuditMavenCommand) runMvn(buildConfiguration *utils.BuildConfiguration) error {
	goals := []string{"-B", "dependency:resolve"}
	log.Debug(fmt.Sprintf("mvn command goals: %v", goals))
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Maven)
	if err != nil {
		return err
	}
	if exists {
		log.Debug("Using resolver config from " + configFilePath)
	} else {
		configFilePath = ""
	}
	return mvnutils.RunMvn(configFilePath, "", buildConfiguration, goals, 0, auditCmd.insecureTls, true)
}

func (na *AuditMavenCommand) CommandName() string {
	return "xr_audit_mvn"
}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	fmt.Println(string(jsonOut))
	return err
}
