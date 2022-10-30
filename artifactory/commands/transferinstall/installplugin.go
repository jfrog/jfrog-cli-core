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
	"path/filepath"
)

const (
	minArtifactoryVersion = "2.9.0" // for reload api (version api is from 2.2.2)
	pluginReloadRestApi   = "api/plugins/reload"
	jHomeEnvVar           = "JFROG_HOME"
	latest                = "[RELEASE]"
)

var (
	defalutSearchPath = filepath.Join("opt", "jfrog")
	// Plugin directory locations
	OriginalDirPath = PluginFileItem{"artifactory", "etc", "plugins"}
	V7DirPath       = PluginFileItem{"artifactory", "var", "etc", "artifactory", "plugins"}
	// Error types
	EmptyUrlErr            = errors.Errorf("Base download URL must be provided to allow file downloads.")
	NotValidDestinationErr = errors.Errorf("Can't find plugin directory with the provided information, this command must run on the Artifactory server.")
	minVerErr              = errorutils.CheckErrorf("This operation requires Artifactory version %s or higher", minArtifactoryVersion)
)

type PluginFileItem []string
type PluginFiles []PluginFileItem

// Get the name componenet of the item
func (f *PluginFileItem) Name() string {
	size := len(*f)
	if size == 0 {
		return ""
	}
	return (*f)[size-1]
}

// Get the directory list of the item, ignore empty entries
func (f *PluginFileItem) Dirs() *PluginFileItem {
	dirs := PluginFileItem{}
	for i := 0; i < len(*f)-1; i++ {
		dir := (*f)[i]
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return &dirs
}

// Split and get the componenets of the item
func (f *PluginFileItem) SplitNameAndDirs() (string, *PluginFileItem) {
	return f.Name(), f.Dirs()
}

// Convert the item to URL representation, ignore empty entries, adding prefix tokens as provided
func (f *PluginFileItem) toURL(previousTokens ...string) string {
	return toURL(toURL(previousTokens...), toURL(*f...))
}

// Convert the item to path representation, ignore empty entries, adding prefix tokens as provided
func (f *PluginFileItem) toPath(previousTokens ...string) string {
	return filepath.Join(filepath.Join(previousTokens...), filepath.Join(*f.Dirs()...), f.Name())
}

// Convert tokens into URL representation, ignore empty ("") entries
func toURL(tokens ...string) string {
	url := ""
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if url != "" {
			url += "/"
		}
		url += token
	}
	return url
}

// Holds all the plugin files and options of destinations to transfer them into
type PluginTransferManager struct {
	files        PluginFiles
	destinations []PluginFileItem
}

// Create new file transfer manager for artifactory plugins
func NewArtifactoryPluginTransferManager(bundle PluginFiles) *PluginTransferManager {
	manager := &PluginTransferManager{
		files:        bundle,
		destinations: []PluginFileItem{},
	}
	// Add all the optional destinations for the plugin dir
	manager.addDestination(OriginalDirPath)
	manager.addDestination(V7DirPath)
	return manager
}

// Add optional plugin directory location as destination
func (ftm *PluginTransferManager) addDestination(directory PluginFileItem) {
	ftm.destinations = append(ftm.destinations, directory)
}

// Search all the local target directories that the plugin directory can exist in base on a given root JFrog artifactory home directory
// the first option that matched and the directory exists is returned as target
func (ftm *PluginTransferManager) trySearchDestinationMatchFrom(rootDir string) (exists bool, target PluginFileItem, err error) {
	if exists, err = fileutils.IsDirExists(rootDir, false); err != nil || !exists {
		return
	}
	exists = false
	for _, optionalPluginDirDst := range ftm.destinations {
		if exists, err = fileutils.IsDirExists(optionalPluginDirDst.toPath(rootDir), false); err != nil {
			return
		}
		if exists {
			target = append([]string{rootDir}, optionalPluginDirDst...)
			return
		}
	}
	return
}

type InstallPluginCommand struct {
	// The name of the plugin the command transfer
	pluginName string
	// The server that the plugin will be installed on
	targetServer *config.ServerDetails
	// Transfer manager to manage files and destinations
	transferManger *PluginTransferManager
	// Source download plugin files information
	installVersion  *version.Version
	baseDownloadUrl string
	// Source local directory to copy from
	localSrcDir string
	// The Jfrog home directory path override as destination
	localJfrogHomePath string
}

// Creeate an InstallPluginCommand
func NewInstallPluginCommand(artifactoryServerDetails *config.ServerDetails, pluginName string, fileTransferManger *PluginTransferManager) *InstallPluginCommand {
	return &InstallPluginCommand{
		targetServer:       artifactoryServerDetails,
		transferManger:     fileTransferManger,
		pluginName:         pluginName,
		installVersion:     nil, // latest
		localSrcDir:        "",
		localJfrogHomePath: "",
	}
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

// Set the base URL that the plugin files avaliable
func (tic *InstallPluginCommand) SetBaseDownloadUrl(baseUrl string) *InstallPluginCommand {
	tic.baseDownloadUrl = baseUrl
	return tic
}

// Set the Jfrog home directory path override to search in it the plugin directory as destination
func (tic *InstallPluginCommand) SetOverrideJfrogHomePath(path string) *InstallPluginCommand {
	tic.localJfrogHomePath = path
	return tic
}

// Validates minimum version and fetch the current
func validateMinArtifactoryVersion(servicesManager artifactory.ArtifactoryServicesManager) (artifactoryVersion *version.Version, err error) {
	log.Debug("verifying install requirements...")
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

// Get the plugin directory destination to install the plugin in it, search is starting from jfrog home directory
func (tic *InstallPluginCommand) getPluginDirDestination() (target PluginFileItem, err error) {
	var exists bool
	var envVal string

	// Flag override
	if tic.localJfrogHomePath != "" {
		log.Debug(fmt.Sprintf("try searching for plugin directory with custom Jfrog home directory '%s'.", tic.localJfrogHomePath))
		if exists, target, err = tic.transferManger.trySearchDestinationMatchFrom(tic.localJfrogHomePath); err != nil || exists {
			return
		}
	}
	// Environment variable override
	if envVal, exists = os.LookupEnv(jHomeEnvVar); exists {
		log.Debug(fmt.Sprintf("try searching for plugin directory with '%s=%s'.", jHomeEnvVar, envVal))
		if exists, target, err = tic.transferManger.trySearchDestinationMatchFrom(envVal); err != nil || exists {
			return
		}
	}
	// Default value
	log.Debug(fmt.Sprintf("try searching for plugin directory with default path '%s'.", defalutSearchPath))
	if exists, target, err = tic.transferManger.trySearchDestinationMatchFrom(defalutSearchPath); err != nil || exists {
		return
	}

	err = NotValidDestinationErr
	return
}

// transfer the bundle from src to dst
type TransferAction func(src string, dst string, bundle PluginFiles) error

// return the src path and a transfer action
func (tic *InstallPluginCommand) getTransferSourceAndAction() (src string, transferAction TransferAction, err error) {
	// check if local directory was provided
	if tic.localSrcDir != "" {
		src = tic.localSrcDir
		transferAction = CopyFiles
		log.Debug("local plugin files provided. copying from file system")
		return
	}
	// make sure base url is set
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
func DownloadFiles(src string, pluginDir string, bundle PluginFiles) (err error) {
	for _, file := range bundle {
		fileName, fileDirs := file.SplitNameAndDirs()
		srcURL := file.toURL(src)
		dstDirPath := fileDirs.toPath(pluginDir)
		log.Debug(fmt.Sprintf("transferring '%s' from '%s' to '%s'", fileName, fileDirs.toURL(src), dstDirPath))
		if err = fileutils.CreateDirIfNotExist(dstDirPath); err != nil {
			return
		}
		if err = downloadutils.DownloadFile(filepath.Join(dstDirPath, fileName), srcURL); err != nil {
			return
		}
	}
	return
}

// Copy the plugin files from the given source to the target directory (create path if needed or override existing files)
func CopyFiles(src string, pluginDir string, bundle PluginFiles) (err error) {
	for _, file := range bundle {
		fileName, fileDirs := file.SplitNameAndDirs()
		srcPath := filepath.Join(src, fileName)
		dstDirPath := fileDirs.toPath(pluginDir)
		log.Debug(fmt.Sprintf("transferring '%s' from '%s' to '%s'", fileName, src, dstDirPath))
		if err = fileutils.CreateDirIfNotExist(dstDirPath); err != nil {
			return
		}
		if err = fileutils.CopyFile(dstDirPath, srcPath); err != nil {
			return
		}
	}
	return
}

// Send reload command to the artifactory in order to reload the plugin files
func sendReLoadCommand(servicesManager artifactory.ArtifactoryServicesManager) error {
	log.Debug("reloading plugins")
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
	log.Info(coreutils.PrintTitle(coreutils.PrintBold(fmt.Sprintf("Installing '%s' plugin...", tic.pluginName))))

	// Initialize and validate
	serviceManager, err := utils.CreateServiceManager(tic.targetServer, -1, 0, false)
	if err != nil {
		return
	}
	if _, err = validateMinArtifactoryVersion(serviceManager); err != nil {
		return
	}
	// Get source, destination and transfer action
	dst, err := tic.getPluginDirDestination()
	if err != nil {
		return
	}
	src, transferAction, err := tic.getTransferSourceAndAction()
	if err != nil {
		return
	}
	// Execute transferring action
	if err = transferAction(src, dst.toPath(), tic.transferManger.files); err != nil {
		return
	}
	// Reload plugins
	if err = sendReLoadCommand(serviceManager); err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("Plugin was installed successfully!.")))
	return nil
}
