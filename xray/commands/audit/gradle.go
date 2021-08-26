package audit

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditGradleCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           OutputFormat
	excludeTestDeps        bool
	useWrapper             bool
	watches                []string
	projectKey             string
	targetRepoPath         string
	includeVulnerabilities bool
	includeLincenses       bool
}

func (auditCmd *AuditGradleCommand) SetServerDetails(server *config.ServerDetails) *AuditGradleCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetOutputFormat(format OutputFormat) *AuditGradleCommand {
	auditCmd.outputFormat = format
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetExcludeTestDeps(excludeTestDeps bool) *AuditGradleCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetUseWrapper(useWrapper bool) *AuditGradleCommand {
	auditCmd.useWrapper = useWrapper
	return auditCmd
}

func (auditCmd *AuditGradleCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func (auditCmd *AuditGradleCommand) SetWatches(watches []string) *AuditGradleCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetProject(project string) *AuditGradleCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetTargetRepoPath(repoPath string) *AuditGradleCommand {
	auditCmd.projectKey = repoPath
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetIncludeVulnerabilities(include bool) *AuditGradleCommand {
	auditCmd.includeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditGradleCommand) SetIncludeLincenses(include bool) *AuditGradleCommand {
	auditCmd.includeLincenses = include
	return auditCmd
}

func NewAuditGradleCommand() *AuditGradleCommand {
	return &AuditGradleCommand{}
}

func (auditCmd *AuditGradleCommand) Run() (err error) {
	// Parse the dependencies into an Xray dependency tree format
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	if err != nil {
		return
	}

	return runScanGraph(modulesDependencyTrees, auditCmd.serverDetails, auditCmd.includeVulnerabilities, auditCmd.includeLincenses, auditCmd.targetRepoPath, auditCmd.projectKey, auditCmd.watches, auditCmd.outputFormat)
}

func (auditCmd *AuditGradleCommand) getModulesDependencyTrees() (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = auditCmd.runGradle(buildConfiguration)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func (auditCmd *AuditGradleCommand) runGradle(buildConfiguration *utils.BuildConfiguration) error {
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
	return gradleutils.RunGradle(tasks, configFilePath, "", buildConfiguration, 0, auditCmd.useWrapper, true)
}

func (na *AuditGradleCommand) CommandName() string {
	return "xr_audit_gradle"
}
