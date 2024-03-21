package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	artifactoryCloudSuffix               = "jfrog.io/artifactory/"
	minCloudImportReleaseBundleSupported = "7.85.0"
)

type ReleaseBundleImportCommand struct {
	releaseBundleCmd
	filePath string
}

func (rbi *ReleaseBundleImportCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbi.serverDetails, nil
}

func (rbi *ReleaseBundleImportCommand) CommandName() string {
	return "rb_import"
}

func NewReleaseBundleImportCommand() *ReleaseBundleImportCommand {
	return &ReleaseBundleImportCommand{}
}
func (rbi *ReleaseBundleImportCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleImportCommand {
	rbi.serverDetails = serverDetails
	return rbi
}

func (rbi *ReleaseBundleImportCommand) SetFilepath(filePath string) *ReleaseBundleImportCommand {
	rbi.filePath = filePath
	return rbi
}

func (rbi *ReleaseBundleImportCommand) Run() (err error) {
	if err = validateArtifactoryVersionSupported(rbi.serverDetails); err != nil {
		return
	}
	artService, err := utils.CreateServiceManager(rbi.serverDetails, 3, 0, false)
	if err != nil {
		return
	}

	exists, err := fileutils.IsFileExists(rbi.filePath, false)
	if !exists || err != nil {
		return
	}

	log.Info("Importing the release bundle archive...")
	if err = artService.ImportReleaseBundle(rbi.filePath); err != nil {
		return
	}
	log.Info("Successfully imported the release bundle archive")
	return
}
