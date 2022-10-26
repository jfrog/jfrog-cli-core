package transferinstall

import (
	"fmt"
	downloadutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const (
	minArtifactoryVersion = "2.9.0" // for reload api (version api is from 2.2.2)
	pluginReloadRestApi   = "api/plugins/reload"
	jHomeEnvVar           = "JFROG_HOME"
	latest                = "[RELEASE]"
)

var (
	// Plugin directory locations
	OriginalDirPath = Directory{"artifactory", "etc", "plugins"}
	V7DirPath       = Directory{"artifactory", "var", "etc", "artifactory", "plugins"}
	// Error types
	NotValidDestinationErr = func(path string) error {
		return errors.Errorf("can't find plugin directory from root directory '%s', must run on machine with artufactory", path)
	}
	EmptyDestinationErr = errors.Errorf("can't find plugin directory")
	EmptyUrlErr         = errors.Errorf("base download URL must be provided for plugin install")
	envVarNotExists     = errorutils.CheckErrorf("The environment variable '%s' must be defined.", jHomeEnvVar)
	minVerErr           = errorutils.CheckErrorf("This operation requires Artifactory version %s or higher", minArtifactoryVersion)
)

type File []string
type FileBundle []File
type Directory []string

func (f *File) Name() string {
	size := len(*f)
	if size < 1 {
		return ""
	}
	return (*f)[size-1]
}

func (f *File) Dirs() []string {
	size := len(*f)
	if size <= 1 {
		return []string{}
	}
	return (*f)[:size-1]
}

func toURL(tokens ...string) string {
	url := ""
	for i, token := range tokens {
		if i > 0 {
			url += "/"
		}
		url += token
	}
	return url
}

// holds all the file information needed to transfer and install plugin
type FileTransferManager struct {
	files        FileBundle
	destinations []Directory
}

func NewFileTransferManager(bundle FileBundle) *FileTransferManager {
	return &FileTransferManager{
		files:        bundle,
		destinations: []Directory{},
	}
}

func NewArtifactoryPluginTransferManager(bundle FileBundle) *FileTransferManager {
	manager := NewFileTransferManager(bundle)
	// Add all the optional destinations for the plugin dir
	manager.addDestination(OriginalDirPath)
	manager.addDestination(V7DirPath)

	return manager
}

func (ftm *FileTransferManager) addDestination(directory Directory) {
	ftm.destinations = append(ftm.destinations, directory)
}

// Search the target directory that the plugins exists in base on a given root JFrog artifactory home directory
// we support the all the directory structures
func (ftm *FileTransferManager) trySearchDestinationMatchFrom(rootDir string) (target Directory, err error) {
	if len(ftm.destinations) == 0 {
		err = EmptyDestinationErr
		return
	}
	// Search artifactory directory structure
	for _, dst := range ftm.destinations {
		exists := false
		exists, err = fileutils.IsDirExists(path.Join(rootDir, path.Join(dst...)), false)
		if err != nil {
			return
		}
		if exists {
			target = append([]string{rootDir}, dst...)
			return
		}
	}
	err = NotValidDestinationErr(rootDir)
	return
}

type InstallPluginCommand struct {
	// The server that the plugin will be installed on
	targetServer *config.ServerDetails
	// transfer manager for the plugin
	transferManger *FileTransferManager
	// version flag to download
	installVersion *version.Version
	// base url that the plugin files available from
	baseDownloadUrl string
	// local directory to copy from
	localSrcDir string
}

func (tic *InstallPluginCommand) ServerDetails() (*config.ServerDetails, error) {
	return tic.targetServer, nil
}

// Set the local directory that the plugin files will be copied from
func (tic *InstallPluginCommand) SetLocalPluginFiles(localDir string) *InstallPluginCommand {
	tic.localSrcDir = localDir
	return tic
}

// Set the plugin version we want to download
func (tic *InstallPluginCommand) SetInstallVersion(installVersion *version.Version) *InstallPluginCommand {
	tic.installVersion = installVersion
	return tic
}

// Set the base URL that the plugin files reside in
func (tic *InstallPluginCommand) SetBaseDownloadUrl(baseUrl string) *InstallPluginCommand {
	tic.baseDownloadUrl = baseUrl
	return tic
}

// Creeate an InstallPluginCommand
func NewInstallPluginCommand(server *config.ServerDetails, transferManger *FileTransferManager) *InstallPluginCommand {
	return &InstallPluginCommand{
		targetServer:   server,
		transferManger: transferManger,
		installVersion: nil, // latest
		localSrcDir:    "",
	}
}

// Validates minimum version and fetch current (for reload API request), JFROG_HOME env var exists
func validateAndFetchArtifactoryVersion(servicesManager artifactory.ArtifactoryServicesManager) (artifactoryVersion *version.Version, err error) {
	// validate version and make sure server is up
	rawVersion, err := servicesManager.GetVersion()
	if err != nil {
		return
	}
	artifactoryVersion = version.NewVersion(rawVersion)
	if !artifactoryVersion.AtLeast(minArtifactoryVersion) {
		err = minVerErr
		return
	}
	return
}

// Validates JFROG_HOME env var exists and fetch it
func validateAndFetchRootPath() (rootPath string, err error) {
	// validate environment variable
	rootPath, exists := os.LookupEnv(jHomeEnvVar)
	if !exists {
		err = envVarNotExists
		return
	}
	return
}

// Validates the information needed for this command.
func validateInstallRequirements(servicesManager artifactory.ArtifactoryServicesManager) (artifactoryVersion *version.Version, rootPath string, err error) {
	artifactoryVersion, err = validateAndFetchArtifactoryVersion(servicesManager)
	if err != nil {
		return
	}
	rootPath, err = validateAndFetchRootPath()
	return
}

// transfer the bundle from src to dst
type TransferAction func(src string, dst string, bundle FileBundle) error

// return the src path and a transfer action
func (tic *InstallPluginCommand) getTransferBundle() (src string, transferAction TransferAction, err error) {
	// check if local directory was provided
	if tic.localSrcDir != "" {
		src = tic.localSrcDir
		transferAction = CopyFiles
		log.Debug("local plugin files provided. copying from file system")
		return
	}
	if tic.baseDownloadUrl == "" {
		err = EmptyUrlErr
		return
	}
	// download file from web
	src = tic.baseDownloadUrl
	if tic.installVersion == nil {
		// Latest
		src = toURL(src, latest)
		log.Debug("fetching latest version to the target.")
	} else {
		src = toURL(src, tic.installVersion.GetVersion())
		log.Debug(fmt.Sprintf("fetching plugin version '%s' to the target.", tic.installVersion.GetVersion()))
	}
	transferAction = DownloadFiles

	return
}

// Download the plugin files from the given url to the target directory (create path if needed or override existing files)
func DownloadFiles(src string, pluginDir string, bundle FileBundle) (err error) {
	for _, file := range bundle {
		srcURL := toURL(src, toURL(file...))
		dstDir := path.Join(pluginDir, path.Join(file.Dirs()...))
		name := file.Name()
		dstPath := path.Join(dstDir, name)
		log.Debug(fmt.Sprintf("transferring '%s' from '%s' to '%s'", name, srcURL, dstDir))
		if err = fileutils.CreateDirIfNotExist(dstDir); err != nil {
			return
		}
		if err = downloadutils.DownloadFile(dstPath, srcURL); err != nil {
			return
		}
	}
	return
}

// Copy the plugin files from the given source to the target directory (create path if needed or override existing files)
func CopyFiles(src string, pluginDir string, bundle FileBundle) (err error) {
	for _, file := range bundle {
		srcPath := filepath.Join(src, file.Name()) // in local copy, all the files are at the same src dir, no need for all local dirs path
		dstDir := filepath.Join(pluginDir, path.Join(file.Dirs()...))
		name := file.Name()
		log.Debug(fmt.Sprintf("transferring '%s' from '%s' to '%s'", name, srcPath, dstDir))
		if err = fileutils.CreateDirIfNotExist(dstDir); err != nil {
			return
		}
		if err = fileutils.CopyFile(dstDir, srcPath); err != nil {
			return
		}
	}
	return
}

// Send reload command to the artifactory in order to reload the plugin files
func sendReLoadCommand(servicesManager artifactory.ArtifactoryServicesManager) error {
	serviceDetails := servicesManager.GetConfig().GetServiceDetails()
	httpDetails := serviceDetails.CreateHttpClientDetails()

	resp, body, err := servicesManager.Client().SendPost(serviceDetails.GetUrl()+pluginReloadRestApi, []byte{}, &httpDetails)
	if err != nil {
		return err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return err
	}
	return nil
}

func (tic *InstallPluginCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold(fmt.Sprintf("Installing plugin..."))))

	// Phase 0: initialize and validate
	serviceManager, err := utils.CreateServiceManager(tic.targetServer, -1, 0, false)
	if err != nil {
		return
	}
	log.Debug("verifying environment and artifactory server...")
	_, rootDir, err := validateInstallRequirements(serviceManager)
	if err != nil {
		return
	}
	// Phase 1: check if there is a matched root in destinations
	log.Debug(fmt.Sprintf("searching inside '%s' for plugin directory", rootDir))
	pluginDir, err := tic.transferManger.trySearchDestinationMatchFrom(rootDir)
	if err != nil {
		return
	}
	// Phase 2: get source transfer bundle and execute transferring plugin files
	src, transferAction, err := tic.getTransferBundle()
	if err != nil {
		return
	}
	if err = transferAction(src, path.Join(pluginDir...), tic.transferManger.files); err != nil {
		return
	}
	// Phase 3: Reload
	log.Debug("plugin files fetched to the target directory, reloading plugins")
	if err = sendReLoadCommand(serviceManager); err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("Plugin was installed successfully!.")))
	return nil
}
