package audit

import (
	"encoding/json"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/common/commands/mvn"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditMavenCommand struct {
	serverDetails   *config.ServerDetails
	excludeTestDeps bool
	insecureTls     bool
}

func (auditCmd *AuditMavenCommand) SetServerDetails(server *config.ServerDetails) *AuditMavenCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetExcludeTestDeps(excludeTestDeps bool) *AuditMavenCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *AuditMavenCommand) SetInsecureTls(insecureTls bool) *AuditMavenCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *AuditMavenCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
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

	return runScanGraph(modulesDependencyTrees, auditCmd.serverDetails)
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
	goals := []string{"-B", "compile"}
	if !auditCmd.excludeTestDeps {
		goals = append(goals, "test-compile")
	}
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
	return mvn.RunMvn(configFilePath, "", buildConfiguration, goals, 0, auditCmd.insecureTls, true)
}

func (na *AuditMavenCommand) CommandName() string {
	return "xr_audit_mvn"
}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	fmt.Println(string(jsonOut))
	return err
}
