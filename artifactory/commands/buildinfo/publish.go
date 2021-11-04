package buildinfo

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	biconf "github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type BuildPublishCommand struct {
	buildConfiguration *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	config             *biconf.Configuration
	detailedSummary    bool
	summary            *clientutils.Sha256Summary
}

func NewBuildPublishCommand() *BuildPublishCommand {
	return &BuildPublishCommand{}
}

func (bpc *BuildPublishCommand) SetConfig(config *biconf.Configuration) *BuildPublishCommand {
	bpc.config = config
	return bpc
}

func (bpc *BuildPublishCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildPublishCommand {
	bpc.serverDetails = serverDetails
	return bpc
}

func (bpc *BuildPublishCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildPublishCommand {
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
	servicesManager, err := utils.CreateServiceManager(bpc.serverDetails, -1, bpc.config.DryRun)
	if err != nil {
		return err
	}

	buildInfoService := utils.CreateBuildInfoService()
	build, err := buildInfoService.GetOrCreateBuildWithProject(bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber, bpc.buildConfiguration.Project)
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
	summary, err := servicesManager.PublishBuildInfo(buildInfo, bpc.buildConfiguration.Project)
	if bpc.IsDetailedSummary() {
		bpc.SetSummary(summary)
	}
	if err != nil {
		return err
	}

	buildLink, err := bpc.constructBuildInfoUiUrl(servicesManager, buildInfo.Started)
	if err != nil {
		return err
	}
	log.Info("Build info successfully deployed.")
	log.Info("Browse it in Artifactory under " + buildLink)

	if !bpc.config.DryRun {
		return build.Clean()
	}
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
	return bpc.getBuildInfoUiUrl(majorVersion, buildTime), nil
}

func (bpc *BuildPublishCommand) getBuildInfoUiUrl(majorVersion int, buildTime time.Time) string {
	if majorVersion <= 6 {
		return fmt.Sprintf("%vartifactory/webapp/#/builds/%v/%v",
			bpc.serverDetails.GetUrl(), bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber)
	} else if bpc.buildConfiguration.Project != "" {
		timestamp := buildTime.UnixNano() / 1000000
		return fmt.Sprintf("%vui/builds/%v/%v/%v/published?buildRepo=%v-build-info&projectKey=%v",
			bpc.serverDetails.GetUrl(), bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber, strconv.FormatInt(timestamp, 10), bpc.buildConfiguration.Project, bpc.buildConfiguration.Project)
	}
	timestamp := buildTime.UnixNano() / 1000000
	return fmt.Sprintf("%vui/builds/%v/%v/%v/published?buildRepo=artifactory-build-info",
		bpc.serverDetails.GetUrl(), bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber, strconv.FormatInt(timestamp, 10))
}
