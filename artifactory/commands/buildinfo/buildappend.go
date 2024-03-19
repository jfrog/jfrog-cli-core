package buildinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/artifactory"
	servicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAppendCommand struct {
	buildConfiguration  *build.BuildConfiguration
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
	if err = build.SaveBuildGeneralDetails(buildName, buildNumber, bac.buildConfiguration.GetProject()); err != nil {
		return err
	}

	// Create services manager to get build-info from Artifactory.
	servicesManager, err := utils.CreateServiceManager(bac.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}

	// Calculate build timestamp
	timestamp, err := bac.getBuildTimestamp(servicesManager)
	if err != nil {
		return err
	}

	// Get checksum values from the build info artifact
	checksumDetails, err := bac.getChecksumDetails(servicesManager, timestamp)
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
	err = build.SavePartialBuildInfo(buildName, buildNumber, bac.buildConfiguration.GetProject(), populateFunc)
	if err == nil {
		log.Info("Build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "successfully appended to", buildName+"/"+buildNumber)
	}
	return err
}

func (bac *BuildAppendCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildAppendCommand {
	bac.serverDetails = serverDetails
	return bac
}

func (bac *BuildAppendCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildAppendCommand {
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
func (bac *BuildAppendCommand) getBuildTimestamp(servicesManager artifactory.ArtifactoryServicesManager) (int64, error) {
	// Get published build-info from Artifactory.
	buildInfoParams := services.BuildInfoParams{BuildName: bac.buildNameToAppend, BuildNumber: bac.buildNumberToAppend, ProjectKey: bac.buildConfiguration.GetProject()}
	buildInfo, found, err := servicesManager.GetBuildInfo(buildInfoParams)
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
func (bac *BuildAppendCommand) getChecksumDetails(servicesManager artifactory.ArtifactoryServicesManager, timestamp int64) (buildinfo.Checksum, error) {
	// Run AQL query for build
	stringTimestamp := strconv.FormatInt(timestamp, 10)
	aqlQuery := servicesutils.CreateAqlQueryForBuildInfoJson(bac.buildConfiguration.GetProject(), bac.buildNameToAppend, bac.buildNumberToAppend, stringTimestamp)
	stream, err := servicesManager.Aql(aqlQuery)
	if err != nil {
		return buildinfo.Checksum{}, err
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(stream.Close()))
	}()

	// Parse AQL results
	aqlResults, err := io.ReadAll(stream)
	if err != nil {
		return buildinfo.Checksum{}, errorutils.CheckError(err)
	}
	parsedResult := new(servicesutils.AqlSearchResult)
	if err = json.Unmarshal(aqlResults, parsedResult); err != nil {
		return buildinfo.Checksum{}, errorutils.CheckError(err)
	}
	if len(parsedResult.Results) == 0 {
		return buildinfo.Checksum{}, errorutils.CheckErrorf("Build '%s/%s' could not be found", bac.buildNameToAppend, bac.buildNumberToAppend)
	}

	// Verify checksum exist
	sha1 := parsedResult.Results[0].Actual_Sha1
	md5 := parsedResult.Results[0].Actual_Md5
	if sha1 == "" || md5 == "" {
		return buildinfo.Checksum{}, errorutils.CheckErrorf("Missing checksums for build-info: '%s/%s', sha1: '%s', md5: '%s'", bac.buildNameToAppend, bac.buildNumberToAppend, sha1, md5)
	}

	// Return checksums
	return buildinfo.Checksum{Sha1: sha1, Md5: md5}, nil
}
