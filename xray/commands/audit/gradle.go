package audit

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/common/commands/gradle"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type XrAuditGradleCommand struct {
	serverDetails   *config.ServerDetails
	excludeTestDeps bool
	useWrapper      bool
}

func (auditCmd *XrAuditGradleCommand) SetServerDetails(server *config.ServerDetails) *XrAuditGradleCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *XrAuditGradleCommand) SetExcludeTestDeps(excludeTestDeps bool) *XrAuditGradleCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *XrAuditGradleCommand) SetUseWrapper(useWrapper bool) *XrAuditGradleCommand {
	auditCmd.useWrapper = useWrapper
	return auditCmd
}

func (auditCmd *XrAuditGradleCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func NewXrAuditGradleCommand() *XrAuditGradleCommand {
	return &XrAuditGradleCommand{}
}

func (auditCmd *XrAuditGradleCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	if err != nil {
		return
	}

	return runScanGraph(modulesDependencyTrees, auditCmd.serverDetails)
}

func (auditCmd *XrAuditGradleCommand) getModulesDependencyTrees() (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = auditCmd.runGradle(buildConfiguration)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func (auditCmd *XrAuditGradleCommand) runGradle(buildConfiguration *utils.BuildConfiguration) error {
	tasks := "clean compileJava "
	if !auditCmd.excludeTestDeps {
		tasks += "compileTestJava "
	}
	tasks += "artifactoryPublish"
	log.Debug(fmt.Sprintf("gradle command tasks: %v", tasks))
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Gradle)
	if err != nil {
		return err
	}
	if exists {
		log.Debug("Using resolver config from " + configFilePath)
	} else {
		configFilePath = ""
	}
	return gradle.RunGradle(tasks, configFilePath, "", buildConfiguration, 0, auditCmd.useWrapper, true)
}

func (na *XrAuditGradleCommand) CommandName() string {
	return "xr_audit_gradle"
}
