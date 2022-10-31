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
	url "net/url"
	"os"
	"path"
	"path/filepath"
)

const (
	minArtifactoryVersion = "2.9.0" // for reload api (version api is from 2.2.2)
	pluginReloadRestApi   = "api/plugins/reload"
	jfrogHomeEnvVar       = "JFROG_HOME"
	latest                = "[RELEASE]"
)

var (
	defaultSearchPath = filepath.Join("opt", "jfrog")
	// Plugin directory locations
	originalDirPath = PluginFileItem{"artifactory", "etc", "plugins"}
	v7DirPath       = PluginFileItem{"artifactory", "var", "etc", "artifactory", "plugins"}
	// Error types
	emptyUrlErr            = errors.Errorf("Base download URL must be provided to allow file downloads.")
	notValidDestinationErr = func(showHint bool) error {
		hint := ""
		if showHint {
			hint = ", this command must run on a machine with Artifactory server. Hint: use --home-dir option."
		}
		return errors.Errorf("Can't find target plugin directory, this command must run on a machine with Artifactory server. %s", hint)
	}
	minVerErr = errorutils.CheckErrorf("This operation requires Artifactory version %s or higher", minArtifactoryVersion)
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
func (f *PluginFileItem) toURL(prefixUrl string) (string, error) {
	myUrl, err := url.Parse(prefixUrl)
	if err != nil {
		return "", err
	}
	myUrl.Path = path.Join(myUrl.Path, path.Join(*f...))
	return myUrl.String(), nil
}

// Convert the item to path representation, ignore empty entries, adding prefix tokens as provided
func (f *PluginFileItem) toPath(previousTokens ...string) string {
	return filepath.Join(filepath.Join(previousTokens...), filepath.Join(*f.Dirs()...), f.Name())
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
	manager.addDestination(originalDirPath)
	manager.addDestination(v7DirPath)
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

func (ipc *InstallPluginCommand) ServerDetails() (*config.ServerDetails, error) {
	return ipc.targetServer, nil
}

// Set the local directory that the plugin files will be copied from
func (ipc *InstallPluginCommand) SetLocalPluginFiles(localDir string) *InstallPluginCommand {
	ipc.localSrcDir = localDir
	return ipc
}

// Set the plugin version we want to download
func (ipc *InstallPluginCommand) SetInstallVersion(installVersion *version.Version) *InstallPluginCommand {
	ipc.installVersion = installVersion
	return ipc
}

// Set the base URL that the plugin files avaliable
func (ipc *InstallPluginCommand) SetBaseDownloadUrl(baseUrl string) *InstallPluginCommand {
	ipc.baseDownloadUrl = baseUrl
	return ipc
}

// Set the Jfrog home directory path override to search in it the plugin directory as destination
func (ipc *InstallPluginCommand) SetJFrogHomePath(path string) *InstallPluginCommand {
	ipc.localJfrogHomePath = path
	return ipc
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
func (ipc *InstallPluginCommand) getPluginDirDestination() (target PluginFileItem, err error) {
	var exists bool
	var envVal string

	// Flag override
	if ipc.localJfrogHomePath != "" {
		log.Debug(fmt.Sprintf("Searching for plugins directory in the JFrog home directory '%s'.", ipc.localJfrogHomePath))
		if exists, target, err = ipc.transferManger.trySearchDestinationMatchFrom(ipc.localJfrogHomePath); err != nil || exists {
			return
		}
		if !exists {
			err = notValidDestinationErr(false)
			return
		}
	}
	// Environment variable override
	if envVal, exists = os.LookupEnv(jfrogHomeEnvVar); exists {
		log.Debug(fmt.Sprintf("Searching for plugins directory in the JFrog home directory '%s' retrieved from the '%s' environment variable.", envVal, jfrogHomeEnvVar))
		if exists, target, err = ipc.transferManger.trySearchDestinationMatchFrom(envVal); err != nil || exists {
			return
		}
	}
	// Default value
	if !coreutils.IsWindows() {
		log.Debug(fmt.Sprintf("Searching for plugins directory in the default path '%s'.", defaultSearchPath))
		if exists, target, err = ipc.transferManger.trySearchDestinationMatchFrom(defaultSearchPath); err != nil || exists {
			return
		}
	}

	err = notValidDestinationErr(true)
	return
}

// transfer the bundle from src to dst
type TransferAction func(src string, dst string, bundle PluginFiles) error

// return the src path and a transfer action
func (ipc *InstallPluginCommand) getTransferSourceAndAction() (src string, transferAction TransferAction, err error) {
	// check if local directory was provided
	if ipc.localSrcDir != "" {
		src = ipc.localSrcDir
		transferAction = CopyFiles
		log.Debug("local plugin files provided. copying from file system")
		return
	}
	// make sure base url is set
	if ipc.baseDownloadUrl == "" {
		err = emptyUrlErr
		return
	}
	var baseSrc *url.URL
	if baseSrc, err = url.Parse(ipc.baseDownloadUrl); err != nil {
		return
	}
	// download file from web
	if ipc.installVersion == nil {
		// Latest
		src = path.Join(baseSrc.Path, latest)
		log.Debug("fetching latest version to the target.")
	} else {
		src = path.Join(baseSrc.Path, ipc.installVersion.GetVersion())
		log.Debug(fmt.Sprintf("fetching plugin version '%s' to the target.", ipc.installVersion.GetVersion()))
	}
	transferAction = DownloadFiles

	return
}

// Download the plugin files from the given url to the target directory (create path if needed or override existing files)
func DownloadFiles(src string, pluginDir string, bundle PluginFiles) (err error) {
	for _, file := range bundle {
		fileName, fileDirs := file.SplitNameAndDirs()
		srcURL, e := file.toURL(src)
		if e != nil {
			return e
		}
		dstDirPath := fileDirs.toPath(pluginDir)
		dirURL, e := fileDirs.toURL(src)
		if e != nil {
			return e
		}
		log.Debug(fmt.Sprintf("transferring '%s' from '%s' to '%s'", fileName, dirURL, dstDirPath))
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
func sendReloadCommand(servicesManager artifactory.ArtifactoryServicesManager) error {
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

func (ipc *InstallPluginCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold(fmt.Sprintf("Installing '%s' plugin...", ipc.pluginName))))

	// Initialize and validate
	serviceManager, err := utils.CreateServiceManager(ipc.targetServer, -1, 0, false)
	if err != nil {
		return
	}
	if _, err = validateMinArtifactoryVersion(serviceManager); err != nil {
		return
	}
	// Get source, destination and transfer action
	dst, err := ipc.getPluginDirDestination()
	if err != nil {
		return
	}
	src, transferAction, err := ipc.getTransferSourceAndAction()
	if err != nil {
		return
	}
	// Execute transferring action
	if err = transferAction(src, dst.toPath(), ipc.transferManger.files); err != nil {
		return
	}
	// Reload plugins
	if err = sendReloadCommand(serviceManager); err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold(fmt.Sprintf("Plugin %s installed successfully in the local Artifactory server.", ipc.pluginName))))
	return nil
}
