package lifecycle

import (
	"encoding/json"
	artUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	artServices "github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	utils2 "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type ReleaseBundleExportCommand struct {
	releaseBundleCmd
	modifications          *services.Modifications
	downloadConfigurations *artUtils.DownloadConfiguration
}

func (rbe *ReleaseBundleExportCommand) Run() (err error) {
	if err = validateArtifactoryVersionSupported(rbe.serverDetails); err != nil {
		return
	}
	servicesManager, rbDetails, queryParams, err := rbe.getPrerequisites()
	if err != nil {
		log.Debug("Failed getting prerequisites for exporting command, error: ", err.Error())
		return
	}
	// Start the Export process and wait for completion
	log.Info("Exporting Release Bundle archive...")
	releaseBundleExportParams := NewReleaseBundleExportParams(rbDetails, *rbe.modifications)
	exportResponse, err := servicesManager.ExportReleaseBundle(releaseBundleExportParams, queryParams)
	if err != nil {
		log.Debug("Failed exporting release bundle, error: ", err.Error())
		return
	}
	// Download the exported bundle
	log.Debug("Downloading the exported bundle...")
	cleanUp, err := rbe.downloadReleaseBundle(exportResponse, rbe.downloadConfigurations)
	defer func() {
		err = cleanUp()
	}()
	if err != nil {
		return
	}
	log.Info("Successfully Downloaded Release Bundle archive")
	content, err := json.Marshal(exportResponse)
	if err != nil {
		return err
	}
	log.Output(utils2.IndentJson(content))
	return
}

// Download the exported release bundle using artifactory service manager
func (rbe *ReleaseBundleExportCommand) downloadReleaseBundle(exportResponse services.ReleaseBundleExportedStatusResponse, downloadConfiguration *artUtils.DownloadConfiguration) (cleanUp func() error, err error) {
	downloadParams := artServices.DownloadParams{
		CommonParams: &utils.CommonParams{
			Pattern: strings.TrimPrefix(exportResponse.RelativeUrl, "/"),
		},
		Symlink:         downloadConfiguration.Symlink,
		ValidateSymlink: downloadConfiguration.ValidateSymlink,
		MinSplitSize:    downloadConfiguration.MinSplitSize,
		SplitCount:      downloadConfiguration.SplitCount,
		SkipChecksum:    downloadConfiguration.SkipChecksum,
	}
	artifactoryServiceManager, err := createArtifactoryServiceManager(rbe.serverDetails)
	if err != nil {
		return
	}
	sum, err := artifactoryServiceManager.DownloadFilesWithSummary(downloadParams)
	return sum.Close, err
}

func NewReleaseBundleExportParams(details services.ReleaseBundleDetails, modifications services.Modifications) (rbExportParams *services.ReleaseBundleExportParams) {
	return &services.ReleaseBundleExportParams{
		ReleaseBundleDetails: services.ReleaseBundleDetails{
			ReleaseBundleName:    details.ReleaseBundleName,
			ReleaseBundleVersion: details.ReleaseBundleVersion,
		},
		Modifications: modifications,
	}
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
	rbe.releaseBundleCmd.serverDetails = serverDetails
	return rbe
}

func (rbe *ReleaseBundleExportCommand) SetReleaseBundleExportModifications(modifications *services.Modifications) *ReleaseBundleExportCommand {
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

func (rbe *ReleaseBundleExportCommand) SetDownloadConfiguration(downloadConfig *artUtils.DownloadConfiguration) *ReleaseBundleExportCommand {
	rbe.downloadConfigurations = downloadConfig
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
