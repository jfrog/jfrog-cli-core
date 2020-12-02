package buildinfo

import (
	"strconv"
	"time"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAppendCommand struct {
	buildConfiguration  *utils.BuildConfiguration
	rtDetails           *config.ArtifactoryDetails
	buildNameToAppend   string
	buildNumberToAppend string
}

func NewBuildAppendCommand() *BuildAppendCommand {
	return &BuildAppendCommand{}
}

func (bac *BuildAppendCommand) CommandName() string {
	return "rt_build_append"
}

func (bac *BuildAppendCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return config.GetDefaultArtifactoryConf()
}

func (bac *BuildAppendCommand) Run() error {
	log.Info("Running Build Append command...")
	timestamp, err := bac.getBuildTimestamp()
	if err != nil {
		return err
	}

	checksumDetails, err := bac.getChecksumDetails(timestamp)
	if err != nil {
		return err
	}
	// if err := utils.SaveBuildGeneralDetails(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber); err != nil {
	// 	return err
	// }

	log.Debug("Appending build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "to build info")
	populateFunc := func(partial *buildinfo.Partial) {
		partial.ModuleType = buildinfo.Build
		partial.ModuleId = bac.buildNameToAppend + "/" + bac.buildNumberToAppend
		partial.Checksum.Sha1 = checksumDetails.Sha1
		partial.Checksum.Md5 = checksumDetails.Md5
	}
	return utils.SavePartialBuildInfo(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber, populateFunc)
}

func (bac *BuildAppendCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *BuildAppendCommand {
	bac.rtDetails = rtDetails
	return bac
}

func (bac *BuildAppendCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildAppendCommand {
	bac.buildConfiguration = buildConfiguration
	return bac
}

func (bac *BuildAppendCommand) SetBuildNameToAppend(buildName string) *BuildAppendCommand {
	bac.buildNameToAppend = buildName
	return bac
}

func (bac *BuildAppendCommand) SetBuildNumberToAppend(buildNumber string) *BuildAppendCommand {
	bac.buildNumberToAppend = buildNumber
	return bac
}

func (bac *BuildAppendCommand) getBuildTimestamp() (int64, error) {
	// Create services manager to get build-info from Artifactory.
	sm, err := utils.CreateServiceManager(bac.rtDetails, false)
	if err != nil {
		return 0, err
	}

	// Get published build-info from Artifactory.
	buildInfoParams := services.BuildInfoParams{BuildName: bac.buildNameToAppend, BuildNumber: bac.buildNumberToAppend}
	buildInfo, err := sm.GetBuildInfo(buildInfoParams)
	if buildInfo.Name == "" {
		return 0, err
	}
	timestamp, err := time.Parse("yyyy-MM-dd'T'HH:mm:ss.SSSZ", buildInfo.Started)
	if err != nil {
		return 0, err
	}
	return timestamp.Unix(), err
}

func (bac *BuildAppendCommand) getChecksumDetails(timestamp int64) (*fileutils.ChecksumDetails, error) {
	serviceDetails, err := bac.rtDetails.CreateArtAuthConfig()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return nil, err
	}

	buildInfoPath := serviceDetails.GetUrl() + "/artifactory-build-info/" + bac.buildNameToAppend + "/" + bac.buildNumberToAppend + "-" + strconv.FormatInt(timestamp, 10) + ".json"
	details, _, err := client.GetRemoteFileDetails(buildInfoPath, serviceDetails.CreateHttpClientDetails())
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &details.Checksum, nil
}
