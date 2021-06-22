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

type XrAuditMavenCommand struct {
	serverDetails   *config.ServerDetails
	excludeTestDeps bool
	insecureTls     bool
}

func (auditCmd *XrAuditMavenCommand) SetServerDetails(server *config.ServerDetails) *XrAuditMavenCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *XrAuditMavenCommand) SetExcludeTestDeps(excludeTestDeps bool) *XrAuditMavenCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *XrAuditMavenCommand) SetInsecureTls(insecureTls bool) *XrAuditMavenCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *XrAuditMavenCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func NewXrAuditMvnCommand() *XrAuditMavenCommand {
	return &XrAuditMavenCommand{}
}

func (auditCmd *XrAuditMavenCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	if err != nil {
		return
	}

	return runScanGraph(modulesDependencyTrees, auditCmd.serverDetails)
}

func (auditCmd *XrAuditMavenCommand) getModulesDependencyTrees() (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = auditCmd.runMvn(buildConfiguration)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func (auditCmd *XrAuditMavenCommand) runMvn(buildConfiguration *utils.BuildConfiguration) error {
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

func (na *XrAuditMavenCommand) CommandName() string {
	return "xr_audit_mvn"
}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	fmt.Println(string(jsonOut))
	return err
}
