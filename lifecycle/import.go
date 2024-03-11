package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ReleaseBundleImportCommand struct {
	releaseBundleCmd
	filePath string
}

func (rbi *ReleaseBundleImportCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbi.serverDetails, nil
}

func (rbi *ReleaseBundleImportCommand) CommandName() string {
	return "rb_export"
}

func NewReleaseBundleImportCommand() *ReleaseBundleImportCommand {
	return &ReleaseBundleImportCommand{}
}
func (rbi *ReleaseBundleImportCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleImportCommand {
	rbi.serverDetails = serverDetails
	rbi.releaseBundleCmd.serverDetails = serverDetails
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
	artService, err := createArtifactoryServiceManager(rbi.serverDetails)
	if err != nil {
		return
	}
	log.Info("Importing Release Bundle Archive...")
	if err = artService.ReleaseBundleImport(rbi.filePath); err != nil {
		return
	}
	log.Info("Successfully Imported Release Bundle archive")
	return
}

func createArtifactoryServiceManager(artDetails *config.ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}
