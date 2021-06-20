package audit

import (
	"encoding/json"

	"github.com/jfrog/jfrog-cli-core/artifactory/commands/buildinfo"
	"github.com/jfrog/jfrog-cli-core/artifactory/commands/mvn"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/xray/commands"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type XrAuditMavenCommand struct {
	serverDetails    *config.ServerDetails
	excludeTestDeps  bool
}

func (auditCmd *XrAuditMavenCommand) SetServerDetails(server *config.ServerDetails) *XrAuditMavenCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *XrAuditMavenCommand) SetExcludeTestDeps(excludeTestDeps bool) *XrAuditMavenCommand {
	auditCmd.excludeTestDeps = excludeTestDeps
	return auditCmd
}

func (auditCmd *XrAuditMavenCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func NewXrAuditMvnCommand() *XrAuditMavenCommand {
	return &XrAuditMavenCommand{}
}

func (auditCmd *XrAuditMavenCommand) Run() (err error) {
	buildConfiguration := &utils.BuildConfiguration{
		BuildName:   "audit-mvn",
		BuildNumber: "1",
	}

	if err = deleteOldBuild(buildConfiguration); err != nil {
		return
	}

	mvnCommand, err := auditCmd.createMvnCommand(buildConfiguration)
	if err != nil {
		return
	}

	err = mvnCommand.Run()
	if err != nil {
		return err
	}

	// Parse the dependencies into an Xray dependency tree format
	// npmGraph := parseNpmDependenciesList(nca.GetDependenciesList(), packageInfo)
	xrayManager, err := commands.CreateXrayServiceManager(auditCmd.serverDetails)
	if err != nil {
		return err
	}
	params := services.NewXrayGraphScanParams()
	// params.Graph = npmGraph
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return err
	}

	scanResults, err := xrayManager.GetScanGraphResults(scanId)
	if err != nil {
		return err
	}
	return printTable(scanResults)

}

func (auditCmd *XrAuditMavenCommand) createMvnCommand(buildConfiguration *utils.BuildConfiguration) (mvnCommand *mvn.MvnCommand, err error) {
	mvnCommand = mvn.NewMvnCommand()
	goals := []string{"compile"}
	if !auditCmd.excludeTestDeps {
		goals = append(goals, "test-compile")
	}
	mvnCommand.SetDisableDeploy(true)
	mvnCommand.SetGoals(goals)
	mvnCommand.SetConfiguration(buildConfiguration)
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Maven)
	if exists {
		mvnCommand.SetConfigPath(configFilePath)
	}
	return
}

func deleteOldBuild(buildConfiguration *utils.BuildConfiguration) error {
	buildClean := buildinfo.NewBuildCleanCommand()
	buildClean.SetBuildConfiguration(buildConfiguration)
	return buildClean.Run()
}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	print(string(jsonOut))
	return err
}

func (na *XrAuditMavenCommand) CommandName() string {
	return "xr_audit_mvn"
}
