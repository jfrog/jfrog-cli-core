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
	"strings"
)

const (
	pluginReloadRestApi = "api/plugins/reload"
	jfrogHomeEnvVar     = "JFROG_HOME"
	latest              = "[RELEASE]"
)

var (
	defaultSearchPath = filepath.Join("opt", "jfrog")
	// Plugin directory locations
	originalDirPath = FileItem{"artifactory", "etc", "plugins"}
	v7DirPath       = FileItem{"artifactory", "var", "etc", "artifactory", "plugins"}
	// Error types
	emptyUrlErr            = errors.Errorf("Base download URL must be provided to allow file downloads.")
	notValidDestinationErr = errorutils.CheckErrorf("Can't find the directory in which to install the data-transfer plugin. Please ensure you're running this command on the machine on which Artifactory is installed. You can also use the --home-dir option to specify the directory.")
	downloadConnectionErr  = func(baseUrl string) error {
		return errorutils.CheckErrorf("We tried to download the plugin files from '%s' but got connection issue. Hint: manual transfer the files to the machine and use --source-dir option.", baseUrl)
	}
)

type InstallPluginCommand struct {
	// The name of the plugin the command will install
	pluginName string
	// The server that the plugin will be installed on
	targetServer *config.ServerDetails
	// install manager manage the list of all the plugin files and the optional target destinations for plugin directory
	transferManger *PluginInstallManager
	// Information for downloading plugin files
	installVersion  *version.Version
	baseDownloadUrl string
	// The local directory the plugin files will be copied from
	localSrcDir string
	// The Jfrog home directory path override as root directory destination from cliutils.InstallPluginHomeDir flag
	localJfrogHomePath string
}

// Creeate an InstallPluginCommand
func NewInstallPluginCommand(artifactoryServerDetails *config.ServerDetails, pluginName string, fileTransferManger *PluginInstallManager) *InstallPluginCommand {
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

// Set the local file system directory path that the plugin files will be copied from
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

// PluginInstallManager holds all the plugin files and optional plugin directory destinations.
// we construct the destination path by searching for the existing plugin directory destination from the options
// and joining the local path of the plugin file with the plugin directory
type PluginInstallManager struct {
	// All the plugin files, represented as FileItem with path starting from inside the plugin directory
	// i.e. for file plugins/lib/file we represent them as FileItem{"lib","file"}
	files PluginFiles
	// All the optional destinations of the plugin directory starting from the JFrog home directory represented as FileItem
	destinations []FileItem
}

// Create new file transfer manager for artifactory plugins
func NewArtifactoryPluginInstallManager(bundle PluginFiles) *PluginInstallManager {
	manager := &PluginInstallManager{
		files:        bundle,
		destinations: []FileItem{},
	}
	// Add all the optional destinations for the plugin dir
	manager.addDestination(originalDirPath)
	manager.addDestination(v7DirPath)
	return manager
}

// Add optional plugin directory location as destination
func (ftm *PluginInstallManager) addDestination(directory FileItem) {
	ftm.destinations = append(ftm.destinations, directory)
}

// Search all the local target directories that the plugin directory can exist in base on a given root JFrog artifactory home directory
// we are searching by Joining rootDir/optionalDestination to find the right structure of the JFrog home directory
// the first option that matched and the directory exists is returned as target
func (ftm *PluginInstallManager) trySearchDestinationMatchFrom(rootDir string) (exists bool, target FileItem, err error) {
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

// Represents a path to file/directory
// i.e. for file dir/dir2/file we represent them as FileItem{"dir","dir2","file"}
// for directory dir/dir2/ we represent them as FileItem{"dir","dir2"}
type FileItem []string

// List of files represented as FileItem
type PluginFiles []FileItem

// Get the name (last) componenet of the item
// i.e. for FileItem{"root","","dir","file.ext"} -> "file.ext"
func (f *FileItem) Name() string {
	size := len(*f)
	if size == 0 {
		return ""
	}
	return (*f)[size-1]
}

// Get the directory FileItem representation of the item, removing the last componenet and ignoring empty entries.
// i.e. for FileItem{"root","","dir","file.ext"} -> FileItem{"root","dir"}
func (f *FileItem) Directories() *FileItem {
	dirs := FileItem{}
	for i := 0; i < len(*f)-1; i++ {
		dir := (*f)[i]
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return &dirs
}

// Split and get the name and directory componenets of the item
func (f *FileItem) SplitNameAndDirs() (string, *FileItem) {
	return f.Name(), f.Directories()
}

// Convert the item to URL representation, adding prefix tokens as provided
func (f *FileItem) toURL(prefixUrl string) (string, error) {
	myUrl, err := url.Parse(prefixUrl)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	myUrl.Path = path.Join(myUrl.Path, path.Join(*f...))
	return myUrl.String(), nil
}

// Convert the item to file path representation, ignoring empty entries, adding prefix tokens as provided
func (f *FileItem) toPath(previousTokens ...string) string {
	return filepath.Join(filepath.Join(previousTokens...), filepath.Join(*f.Directories()...), f.Name())
}

// Get the plugin directory destination to install the plugin in it, search is starting from jfrog home directory
func (ipc *InstallPluginCommand) getPluginDirDestination() (target FileItem, err error) {
	var exists bool
	var envVal string

	// Flag override
	if ipc.localJfrogHomePath != "" {
		log.Debug(fmt.Sprintf("Searching for plugins directory in the JFrog home directory '%s'.", ipc.localJfrogHomePath))
		if exists, target, err = ipc.transferManger.trySearchDestinationMatchFrom(ipc.localJfrogHomePath); err != nil || exists {
			return
		}
		if !exists {
			err = notValidDestinationErr
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

	err = notValidDestinationErr
	return
}

// transfer the file bundle from src to dst
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
		baseSrc.Path = path.Join(baseSrc.Path, latest)
		log.Debug("fetching latest version to the target.")
	} else {
		baseSrc.Path = path.Join(baseSrc.Path, ipc.installVersion.GetVersion())
		log.Debug(fmt.Sprintf("fetching plugin version '%s' to the target.", ipc.installVersion.GetVersion()))
	}
	src = baseSrc.String()
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
			if strings.Contains(err.Error(), "TLS handshake timeout") {
				err = downloadConnectionErr(src)
			}
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

	// Initialize
	serviceManager, err := utils.CreateServiceManager(ipc.targetServer, -1, 0, false)
	if err != nil {
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
