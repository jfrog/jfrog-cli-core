package buildinfo

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	buildinfo "github.com/jfrog/build-info-go/entities"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAppendCommand struct {
	buildConfiguration  *utils.BuildConfiguration
	serverDetails       *config.ServerDetails
	buildNameToAppend   string
	buildNumberToAppend string
}

func NewBuildAppendCommand() *BuildAppendCommand {
	return &BuildAppendCommand{}
}

func (bac *BuildAppendCommand) CommandName() string {
	return "rt_build_append"
}

func (bac *BuildAppendCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetDefaultServerConf()
}

func (bac *BuildAppendCommand) Run() error {
	log.Info("Running Build Append command...")
	buildName, err := bac.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bac.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber, bac.buildConfiguration.GetProject()); err != nil {
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
		partial.Checksum = buildinfo.Checksum{
			Sha1: checksumDetails.Sha1,
			Md5:  checksumDetails.Md5,
		}
	}
	err = utils.SavePartialBuildInfo(buildName, buildNumber, bac.buildConfiguration.GetProject(), populateFunc)
	if err == nil {
		log.Info("Build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "successfully appended to", buildName+"/"+buildNumber)
	}
	return err
}

func (bac *BuildAppendCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildAppendCommand {
	bac.serverDetails = serverDetails
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
	sm, err := utils.CreateServiceManager(bac.serverDetails, -1, 0, false)
	if err != nil {
		return 0, err
	}

	// Get published build-info from Artifactory.
	buildInfoParams := services.BuildInfoParams{BuildName: bac.buildNameToAppend, BuildNumber: bac.buildNumberToAppend, ProjectKey: bac.buildConfiguration.GetProject()}
	buildInfo, found, err := sm.GetBuildInfo(buildInfoParams)
	if err != nil {
		return 0, err
	}
	buildString := fmt.Sprintf("Build %s/%s", bac.buildNameToAppend, bac.buildNumberToAppend)
	if bac.buildConfiguration.GetProject() != "" {
		buildString = buildString + " of project: " + bac.buildConfiguration.GetProject()
	}
	if !found {
		return 0, errorutils.CheckErrorf(buildString + " not found in Artifactory.")
	}

	buildTime, err := time.Parse(buildinfo.TimeFormat, buildInfo.BuildInfo.Started)
	if errorutils.CheckError(err) != nil {
		return 0, err
	}

	// Convert from nanoseconds to milliseconds
	timestamp := buildTime.UnixNano() / 1000000
	log.Debug(buildString + ". Started: " + buildInfo.BuildInfo.Started + ". Calculated timestamp: " + strconv.FormatInt(timestamp, 10))

	return timestamp, err
}

// Download MD5 and SHA1 from the build info artifact.
func (bac *BuildAppendCommand) getChecksumDetails(timestamp int64) (buildinfo.Checksum, error) {
	serviceDetails, err := bac.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return buildinfo.Checksum{}, err
	}
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return buildinfo.Checksum{}, err
	}

	buildInfoRepo := "artifactory-build-info"
	if bac.buildConfiguration.GetProject() != "" {
		buildInfoRepo = url.PathEscape(bac.buildConfiguration.GetProject()) + "-build-info"
	}
	buildInfoPath := fmt.Sprintf("%v%v/%v/%v-%v.json", serviceDetails.GetUrl(), buildInfoRepo, url.PathEscape(bac.buildNameToAppend), url.PathEscape(bac.buildNumberToAppend), strconv.FormatInt(timestamp, 10))
	details, resp, err := client.GetRemoteFileDetails(buildInfoPath, serviceDetails.CreateHttpClientDetails())
	if err != nil {
		return buildinfo.Checksum{}, err
	}
	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return buildinfo.Checksum{}, err
	}
	log.Debug("Artifactory response:", resp.Status)

	return details.Checksum, nil
}
