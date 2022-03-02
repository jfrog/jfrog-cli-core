package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditGradleCommand struct {
	audit.AuditCommand
	excludeTestDeps bool
	useWrapper      bool
}

func NewEmptyAuditGradleCommand() *AuditGradleCommand {
	return &AuditGradleCommand{AuditCommand: *audit.NewAuditCommand()}
}

func NewAuditGradleCommand(auditCmd audit.AuditCommand) *AuditGradleCommand {
	return &AuditGradleCommand{AuditCommand: auditCmd}
}

func (auditCmd *AuditGradleCommand) SetExcludeTestDeps(excludeTestDeps bool) *AuditGradleCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetUseWrapper(useWrapper bool) *AuditGradleCommand {
	auditCmd.useWrapper = useWrapper
	return auditCmd
}

func (auditCmd *AuditGradleCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := BuildGradleDependencyTree(auditCmd.excludeTestDeps, auditCmd.useWrapper)
	if err != nil {
		return
	}

	return auditCmd.ScanDependencyTree(modulesDependencyTrees)
}

func BuildGradleDependencyTree(excludeTestDeps, useWrapper bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = runGradle(buildConfiguration, excludeTestDeps, useWrapper)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runGradle(buildConfiguration *utils.BuildConfiguration, excludeTestDeps, useWrapper bool) error {
	tasks := "clean compileJava "
	if !excludeTestDeps {
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
	return gradleutils.RunGradle(tasks, configFilePath, "", buildConfiguration, 0, useWrapper, true)
}

func (auditCmd *AuditGradleCommand) CommandName() string {
	return "xr_audit_gradle"
}
