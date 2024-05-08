package buildinfo

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/formats"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	biconf "github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artclientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildPublishCommand struct {
	buildConfiguration *build.BuildConfiguration
	serverDetails      *config.ServerDetails
	config             *biconf.Configuration
	detailedSummary    bool
	summary            *clientutils.Sha256Summary
	githubSummary      utils.GitHubActionSummaryImpl
}

func NewBuildPublishCommand() *BuildPublishCommand {
	return &BuildPublishCommand{githubSummary: *utils.NewBuildPublishGithubSummary()}
}

func (bpc *BuildPublishCommand) SetConfig(config *biconf.Configuration) *BuildPublishCommand {
	bpc.config = config
	return bpc
}

func (bpc *BuildPublishCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildPublishCommand {
	bpc.serverDetails = serverDetails
	return bpc
}

func (bpc *BuildPublishCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildPublishCommand {
	bpc.buildConfiguration = buildConfiguration
	return bpc
}

func (bpc *BuildPublishCommand) SetSummary(summary *clientutils.Sha256Summary) *BuildPublishCommand {
	bpc.summary = summary
	return bpc
}

func (bpc *BuildPublishCommand) GetSummary() *clientutils.Sha256Summary {
	return bpc.summary
}

func (bpc *BuildPublishCommand) SetDetailedSummary(detailedSummary bool) *BuildPublishCommand {
	bpc.detailedSummary = detailedSummary
	return bpc
}

func (bpc *BuildPublishCommand) IsDetailedSummary() bool {
	return bpc.detailedSummary
}

func (bpc *BuildPublishCommand) CommandName() string {
	return "rt_build_publish"
}

func (bpc *BuildPublishCommand) ServerDetails() (*config.ServerDetails, error) {
	return bpc.serverDetails, nil
}

func (bpc *BuildPublishCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(bpc.serverDetails, -1, 0, bpc.config.DryRun)
	if err != nil {
		return err
	}

	buildInfoService := build.CreateBuildInfoService()
	buildName, err := bpc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bpc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	build, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, bpc.buildConfiguration.GetProject())
	if errorutils.CheckError(err) != nil {
		return err
	}

	build.SetAgentName(coreutils.GetCliUserAgentName())
	build.SetAgentVersion(coreutils.GetCliUserAgentVersion())
	build.SetBuildAgentVersion(coreutils.GetClientAgentVersion())
	build.SetPrincipal(bpc.serverDetails.User)
	build.SetBuildUrl(bpc.config.BuildUrl)

	buildInfo, err := build.ToBuildInfo()
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = buildInfo.IncludeEnv(strings.Split(bpc.config.EnvInclude, ";")...)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = buildInfo.ExcludeEnv(strings.Split(bpc.config.EnvExclude, ";")...)
	if errorutils.CheckError(err) != nil {
		return err
	}
	if bpc.buildConfiguration.IsLoadedFromConfigFile() {
		buildInfo.Number, err = bpc.getNextBuildNumber(buildInfo.Name, servicesManager)
		if errorutils.CheckError(err) != nil {
			return err
		}
		bpc.buildConfiguration.SetBuildNumber(buildInfo.Number)
	}
	summary, err := servicesManager.PublishBuildInfo(buildInfo, bpc.buildConfiguration.GetProject())
	if bpc.IsDetailedSummary() {
		bpc.SetSummary(summary)
	}
	if err != nil || bpc.config.DryRun {
		return err
	}

	buildLink, err := bpc.constructBuildInfoUiUrl(servicesManager, buildInfo.Started)
	if err != nil {
		return err
	}

	err = build.Clean()
	if err != nil {
		return err
	}

	// Save build info data to display in GitHub Job Summary if needed
	buildInfo.BuildUrl = buildLink
	if err = bpc.githubSummary.RecordCommandOutput(buildInfo); err != nil {
		log.Warn("failed to collect build info data for GitHub Job Summary: ", err)
	}

	logMsg := "Build info successfully deployed."
	if bpc.IsDetailedSummary() {
		log.Info(logMsg + " Browse it in Artifactory under " + buildLink)
		return nil
	}

	log.Info(logMsg)
	return logJsonOutput(buildLink)
}

func logJsonOutput(buildInfoUiUrl string) error {
	output := formats.BuildPublishOutput{BuildInfoUiUrl: buildInfoUiUrl}
	results, err := output.JSON()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Output(clientutils.IndentJson(results))
	return nil
}

func (bpc *BuildPublishCommand) constructBuildInfoUiUrl(servicesManager artifactory.ArtifactoryServicesManager, buildInfoStarted string) (string, error) {
	buildTime, err := time.Parse(buildinfo.TimeFormat, buildInfoStarted)
	if errorutils.CheckError(err) != nil {
		return "", err
	}
	artVersion, err := servicesManager.GetVersion()
	if err != nil {
		return "", err
	}
	artVersionSlice := strings.Split(artVersion, ".")
	majorVersion, err := strconv.Atoi(artVersionSlice[0])
	if errorutils.CheckError(err) != nil {
		return "", err
	}
	return bpc.getBuildInfoUiUrl(majorVersion, buildTime)
}

func (bpc *BuildPublishCommand) getBuildInfoUiUrl(majorVersion int, buildTime time.Time) (string, error) {
	buildName, err := bpc.buildConfiguration.GetBuildName()
	if err != nil {
		return "", err
	}
	buildNumber, err := bpc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return "", err
	}

	baseUrl := bpc.serverDetails.GetUrl()
	if baseUrl == "" {
		baseUrl = strings.TrimSuffix(strings.TrimSuffix(bpc.serverDetails.GetArtifactoryUrl(), "/"), "artifactory")
	}
	baseUrl = clientutils.AddTrailingSlashIfNeeded(baseUrl)

	project := bpc.buildConfiguration.GetProject()
	buildName, buildNumber, project = url.PathEscape(buildName), url.PathEscape(buildNumber), url.QueryEscape(project)

	if majorVersion <= 6 {
		return fmt.Sprintf("%vartifactory/webapp/#/builds/%v/%v",
			baseUrl, buildName, buildNumber), nil
	}
	timestamp := buildTime.UnixMilli()
	if project != "" {
		return fmt.Sprintf("%vui/builds/%v/%v/%v/published?buildRepo=%v-build-info&projectKey=%v",
			baseUrl, buildName, buildNumber, strconv.FormatInt(timestamp, 10), project, project), nil
	}
	return fmt.Sprintf("%vui/builds/%v/%v/%v/published?buildRepo=artifactory-build-info",
		baseUrl, buildName, buildNumber, strconv.FormatInt(timestamp, 10)), nil
}

// Return the next build number based on the previously published build.
// Return "1" if no build is found
func (bpc *BuildPublishCommand) getNextBuildNumber(buildName string, servicesManager artifactory.ArtifactoryServicesManager) (string, error) {
	publishedBuildInfo, found, err := servicesManager.GetBuildInfo(services.BuildInfoParams{BuildName: buildName, BuildNumber: artclientutils.LatestBuildNumberKey})
	if err != nil {
		return "", err
	}
	if !found || publishedBuildInfo.BuildInfo.Number == "" {
		return "1", nil
	}
	latestBuildNumber, err := strconv.Atoi(publishedBuildInfo.BuildInfo.Number)
	if errorutils.CheckError(err) != nil {
		if errors.Is(err, strconv.ErrSyntax) {
			log.Warn("The latest build number is " + publishedBuildInfo.BuildInfo.Number + ". Since it is not an integer, and therefore cannot be incremented to automatically generate the next build number, setting the next build number to 1.")
			return "1", nil
		}
		return "", err
	}
	latestBuildNumber++
	return strconv.Itoa(latestBuildNumber), nil
}
