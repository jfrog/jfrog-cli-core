package audit

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditMavenCommand struct {
	AuditCommand
	insecureTls bool
}

func NewEmptyAuditMavenCommand() *AuditMavenCommand {
	return &AuditMavenCommand{AuditCommand: *NewAuditCommand()}
}

func NewAuditMavenCommand(auditCmd AuditCommand) *AuditMavenCommand {
	return &AuditMavenCommand{AuditCommand: auditCmd}
}

func (auditCmd *AuditMavenCommand) SetInsecureTls(insecureTls bool) *AuditMavenCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *AuditMavenCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	if err != nil {
		return
	}

	return auditCmd.ScanDependencyTree(modulesDependencyTrees)
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
	goals := []string{"-B", "compile", "test-compile"}
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
