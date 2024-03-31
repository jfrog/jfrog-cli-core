package lifecycle

import (
	artUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	artServices "github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type ReleaseBundleExportCommand struct {
	releaseBundleCmd
	modifications          services.Modifications
	downloadConfigurations artUtils.DownloadConfiguration
	targetPath             string
}

func (rbe *ReleaseBundleExportCommand) Run() (err error) {
	if err = validateArtifactoryVersionSupported(rbe.serverDetails); err != nil {
		return
	}
	servicesManager, rbDetails, queryParams, err := rbe.getPrerequisites()
	if err != nil {
		return errorutils.CheckErrorf("Failed getting prerequisites for exporting command, error: '%s'", err.Error())
	}
	// Start the Export process and wait for completion
	log.Info("Exporting Release Bundle archive...")
	exportResponse, err := servicesManager.ExportReleaseBundle(rbDetails, rbe.modifications, queryParams)
	if err != nil {
		return errorutils.CheckErrorf("Failed exporting release bundle, error: '%s'", err.Error())
	}
	// Download the exported bundle
	log.Debug("Downloading the exported bundle...")
	downloaded, failed, err := rbe.downloadReleaseBundle(exportResponse, rbe.downloadConfigurations)
	if err != nil || failed > 0 || downloaded < 1 {
		return
	}
	log.Info("Successfully Downloaded Release Bundle archive")
	return
}

// Download the exported release bundle using artifactory service manager
func (rbe *ReleaseBundleExportCommand) downloadReleaseBundle(exportResponse services.ReleaseBundleExportedStatusResponse, downloadConfiguration artUtils.DownloadConfiguration) (downloaded int, failed int, err error) {
	downloadParams := artServices.DownloadParams{
		CommonParams: &utils.CommonParams{
			Pattern: strings.TrimPrefix(exportResponse.RelativeUrl, "/"),
			Target:  rbe.targetPath,
		},
		MinSplitSize: downloadConfiguration.MinSplitSize,
		SplitCount:   downloadConfiguration.SplitCount,
	}
	artifactoryServiceManager, err := createArtifactoryServiceManager(rbe.serverDetails)
	if err != nil {
		return
	}
	return artifactoryServiceManager.DownloadFiles(downloadParams)

}
func (rbe *ReleaseBundleExportCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbe.serverDetails, nil
}

func (rbe *ReleaseBundleExportCommand) CommandName() string {
	return "rb_export"
}

func NewReleaseBundleExportCommand() *ReleaseBundleExportCommand {
	return &ReleaseBundleExportCommand{}
}
func (rbe *ReleaseBundleExportCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleExportCommand {
	rbe.serverDetails = serverDetails
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetReleaseBundleExportModifications(modifications services.Modifications) *ReleaseBundleExportCommand {
	rbe.modifications = modifications
	return rbe
}
func (rbe *ReleaseBundleExportCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleExportCommand {
	rbe.releaseBundleName = releaseBundleName
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleExportCommand {
	rbe.releaseBundleVersion = releaseBundleVersion
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetProject(project string) *ReleaseBundleExportCommand {
	rbe.rbProjectKey = project
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetDownloadConfiguration(downloadConfig artUtils.DownloadConfiguration) *ReleaseBundleExportCommand {
	rbe.downloadConfigurations = downloadConfig
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetTargetPath(target string) *ReleaseBundleExportCommand {
	if target == "" {
		// Default value as current dir
		target += "./"
	}
	rbe.targetPath = target
	return rbe
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
