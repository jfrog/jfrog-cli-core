package buildinfo

import (
	"errors"
	"net/http"
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
	if err := utils.SaveBuildGeneralDetails(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber); err != nil {
		return err
	}

	// Calculate build timestamp
	timestamp, err := bac.getBuildTimestamp()
	if err != nil {
		return err
	}

	// Get checksum headers from the build info artifact
	checksumDetails, err := bac.getChecksumDetails(timestamp)
	if err != nil {
		return err
	}

	log.Debug("Appending build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "to build info")
	populateFunc := func(partial *buildinfo.Partial) {
		partial.ModuleType = buildinfo.Build
		partial.ModuleId = bac.buildNameToAppend + "/" + bac.buildNumberToAppend
		partial.Checksum = &buildinfo.Checksum{
			Sha1: checksumDetails.Sha1,
			Md5:  checksumDetails.Md5,
		}
	}
	err = utils.SavePartialBuildInfo(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber, populateFunc)
	if err == nil {
		log.Info("Build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "successfully appended to", bac.buildConfiguration.BuildName+"/"+bac.buildConfiguration.BuildNumber)
	}
	return err
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

// Get build timestamp of the build to append. The build timestamp has to be converted to milliseconds from epoch.
// For example, start time of: 2020-11-27T14:33:38.538+0200 should be converted to 1606480418538.
func (bac *BuildAppendCommand) getBuildTimestamp() (int64, error) {
	// Create services manager to get build-info from Artifactory.
	sm, err := utils.CreateServiceManager(bac.rtDetails, false)
	if err != nil {
		return 0, err
	}

	// Get published build-info from Artifactory.
	buildInfoParams := services.BuildInfoParams{BuildName: bac.buildNameToAppend, BuildNumber: bac.buildNumberToAppend}
	buildInfo, found, err := sm.GetBuildInfo(buildInfoParams)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errorutils.CheckError(errors.New("Build " + bac.buildNameToAppend + "/" + bac.buildNumberToAppend + " not found in Artifactory."))
	}

	buildTime, err := time.Parse("2006-01-02T15:04:05.999Z0700", buildInfo.BuildInfo.Started)
	if err != nil {
		return 0, err
	}

	// Convert from nanoseconds to milliseconds
	timestamp := buildTime.UnixNano() / 1000000
	log.Debug("Build " + bac.buildNameToAppend + "/" + bac.buildNumberToAppend + ". Started: " + buildInfo.BuildInfo.Started + ". Calculated timestamp: " + strconv.FormatInt(timestamp, 10))

	return timestamp, err
}

// Download MD5 and SHA1 from the build info artifact.
func (bac *BuildAppendCommand) getChecksumDetails(timestamp int64) (fileutils.ChecksumDetails, error) {
	serviceDetails, err := bac.rtDetails.CreateArtAuthConfig()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return fileutils.ChecksumDetails{}, err
	}

	buildInfoPath := serviceDetails.GetUrl() + "artifactory-build-info/" + bac.buildNameToAppend + "/" + bac.buildNumberToAppend + "-" + strconv.FormatInt(timestamp, 10) + ".json"
	details, resp, err := client.GetRemoteFileDetails(buildInfoPath, serviceDetails.CreateHttpClientDetails())
	if err != nil {
		return fileutils.ChecksumDetails{}, errorutils.CheckError(err)
	}
	log.Debug("Artifactory response: ", resp.Status)
	err = errorutils.CheckResponseStatus(resp, http.StatusOK)
	if errorutils.CheckError(err) != nil {
		return fileutils.ChecksumDetails{}, err
	}

	return details.Checksum, nil
}
