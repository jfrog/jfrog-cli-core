package transferinstall

import (
	"fmt"
	downloadutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	pluginName          = "data-transfer"
	groovyFileName      = "dataTransfer.groovy"
	jarFileName         = "data-transfer.jar"
	dataTransferUrl     = "https://releases.jfrog.io/artifactory/jfrog-releases/data-transfer"
	libDir              = "lib"
	artifactory         = "artifactory"
	pluginReloadRestApi = "api/plugins/reload"
	jfrogHomeEnvVar     = "JFROG_HOME"
	latest              = "[RELEASE]"
)

var (
	defaultSearchPath = filepath.Join("opt", "jfrog")
	// Plugin directory locations
	originalDirPath = FileItem{"etc", "plugins"}
	v7DirPath       = FileItem{"var", "etc", "artifactory", "plugins"}
	// Error types
	notValidDestinationErr = fmt.Errorf("can't find the directory in which to install the data-transfer plugin. Please ensure you're running this command on the machine on which Artifactory is installed. You can also use the --home-dir option to specify the directory.")
	downloadConnectionErr  = func(baseUrl, fileName, err string) error {
		return fmt.Errorf("Could not download the plugin file - '%s' from '%s' due to the following error: '%s'. If this machine has no network access to the download URL, you can download these files from another machine and place them in a directory on this machine. You can then run this command again with the --dir command option, with the directory containing the files as the value.", fileName, baseUrl, err)
	}
	// Plugin files
	transferPluginFiles = PluginFiles{
		FileItem{groovyFileName},
		FileItem{libDir, jarFileName},
	}
)

type InstallDataTransferPluginCommand struct {
	// The server that the plugin will be installed on
	targetServer *config.ServerDetails
	// install manager manages the list of all the plugin files and the optional target destinations for plugin directory
	transferManger *PluginInstallManager
	// The plugin version to download
	installVersion *version.Version
	// The local directory the plugin files will be copied from
	localPluginFilesDir string
	// The JFrog home directory path provided from cliutils.InstallPluginHomeDir flag
	localJFrogHomePath string
}

func (idtp *InstallDataTransferPluginCommand) CommandName() string {
	return "rt_transfer_install"
}

func (idtp *InstallDataTransferPluginCommand) ServerDetails() (*config.ServerDetails, error) {
	return idtp.targetServer, nil
}

// Set the local file system directory path that the plugin files will be copied from
func (idtp *InstallDataTransferPluginCommand) SetLocalPluginFiles(localDir string) *InstallDataTransferPluginCommand {
	idtp.localPluginFilesDir = localDir
	return idtp
}

// Set the plugin version we want to download
func (idtp *InstallDataTransferPluginCommand) SetInstallVersion(installVersion *version.Version) *InstallDataTransferPluginCommand {
	idtp.installVersion = installVersion
	return idtp
}

// Set the Jfrog home directory path
func (idtp *InstallDataTransferPluginCommand) SetJFrogHomePath(path string) *InstallDataTransferPluginCommand {
	idtp.localJFrogHomePath = path
	return idtp
}

// Create InstallDataTransferCommand
func NewInstallDataTransferCommand(server *config.ServerDetails) *InstallDataTransferPluginCommand {
	manager := NewArtifactoryPluginInstallManager(transferPluginFiles)
	cmd := &InstallDataTransferPluginCommand{
		targetServer:   server,
		transferManger: manager,
		// Latest
		installVersion:      nil,
		localPluginFilesDir: "",
		localJFrogHomePath:  "",
	}
	return cmd
}

// PluginInstallManager holds all the plugin files and optional plugin directory destinations.
// We construct the destination path by searching for the existing plugin directory destination from the options
// and joining the local path of the plugin file with the plugin directory
type PluginInstallManager struct {
	// All the plugin files, represented as FileItem with path starting from inside the plugin directory
	// i.e. for file plugins/lib/file we represent them as FileItem{"lib","file"}
	files PluginFiles
	// All the optional destinations of the plugin directory starting from the JFrog home directory represented as FileItem
	destinations []FileItem
}

// Creates a new Artifactory plugin installation manager
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

// Search all the local target directories that the plugin directory can exist in base on a given root JFrog Artifactory home directory
// We are searching by Joining rootDir/optionalDestination to find the right structure of the JFrog home directory.
// The first option that matched and the directory exists is returned as target.
func (ftm *PluginInstallManager) findDestination(rootDir string) (exists bool, destination FileItem, err error) {
	if exists, err = fileutils.IsDirExists(rootDir, false); err != nil || !exists {
		return
	}
	exists = false
	// Search for the product Artifactory folder
	folderItems, err := fileutils.ListFiles(rootDir, true)
	if err != nil {
		return
	}
	for _, folder := range folderItems {
		if strings.Contains(folder, artifactory) {
			// Search for the destination inside the folder
			for _, optionalPluginDirDst := range ftm.destinations {
				if exists, err = fileutils.IsDirExists(optionalPluginDirDst.toPath(folder), false); err != nil {
					return
				}
				if exists {
					destination = append([]string{folder}, optionalPluginDirDst...)
					return
				}
			}
		}
	}
	return
}

// Represents a path to file/directory
// We represent file 'dir/dir2/file' as FileItem{"dir", "dir2", "file"}
// We represent directory 'dir/dir2/' as FileItem{"dir", "dir2"}
type FileItem []string

// List of files represented as FileItem
type PluginFiles []FileItem

// Get the name (last) componenet of the item
// i.e. for FileItem{"root", "", "dir", "file.ext"} -> "file.ext"
func (f *FileItem) Name() string {
	size := len(*f)
	if size == 0 {
		return ""
	}
	return (*f)[size-1]
}

// Get the directory FileItem representation of the item, removing the last componenet and ignoring empty entries.
// i.e. for FileItem{"root", "", "dir", "file.ext"} -> FileItem{"root", "dir"}
func (f *FileItem) Dirs() *FileItem {
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
	return f.Name(), f.Dirs()
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
	return filepath.Join(filepath.Join(previousTokens...), filepath.Join(*f.Dirs()...), f.Name())
}

// Get the plugin directory destination to install the plugin in it. The search is starting from JFrog home directory
func (idtp *InstallDataTransferPluginCommand) getPluginDirDestination() (target FileItem, err error) {
	var exists bool
	var envVal string

	// Flag override
	if idtp.localJFrogHomePath != "" {
		log.Debug(fmt.Sprintf("Searching for the 'plugins' directory in the JFrog home directory '%s'.", idtp.localJFrogHomePath))
		if exists, target, err = idtp.transferManger.findDestination(idtp.localJFrogHomePath); err != nil || exists {
			return
		}
		if !exists {
			err = notValidDestinationErr
			return
		}
	}
	// Environment variable override
	if envVal, exists = os.LookupEnv(jfrogHomeEnvVar); exists {
		log.Debug(fmt.Sprintf("Searching for the 'plugins' directory in the JFrog home directory '%s' retrieved from the '%s' environment variable.", envVal, jfrogHomeEnvVar))
		if exists, target, err = idtp.transferManger.findDestination(envVal); err != nil || exists {
			return
		}
	}
	// Default value
	if !coreutils.IsWindows() {
		log.Debug(fmt.Sprintf("Searching for the 'plugins' directory in the default path '%s'.", defaultSearchPath))
		if exists, target, err = idtp.transferManger.findDestination(defaultSearchPath); err != nil || exists {
			return
		}
	}

	err = notValidDestinationErr
	return
}

// Transfers the file bundle from src to dst
type TransferAction func(src string, dst string, bundle PluginFiles) error

// Returns the src path and a transfer action
func (idtp *InstallDataTransferPluginCommand) getTransferSourceAndAction() (src string, transferAction TransferAction, err error) {
	// Check if local directory was provided
	if idtp.localPluginFilesDir != "" {
		src = idtp.localPluginFilesDir
		transferAction = CopyFiles
		log.Debug("Local plugin files provided, copying from file system.")
		return
	}
	// Download files from web
	var baseSrc *url.URL
	if baseSrc, err = url.Parse(dataTransferUrl); err != nil {
		return
	}
	if idtp.installVersion == nil {
		// Latest
		baseSrc.Path = path.Join(baseSrc.Path, latest)
		log.Debug("Fetching latest version to the target.")
	} else {
		baseSrc.Path = path.Join(baseSrc.Path, idtp.installVersion.GetVersion())
		log.Debug(fmt.Sprintf("Fetching plugin version '%s' to the target.", idtp.installVersion.GetVersion()))
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
		log.Debug(fmt.Sprintf("Downloading '%s' from '%s' to '%s'", fileName, dirURL, dstDirPath))
		if err = fileutils.CreateDirIfNotExist(dstDirPath); err != nil {
			return
		}
		if err = downloadutils.DownloadFile(filepath.Join(dstDirPath, fileName), srcURL); err != nil {
			err = downloadConnectionErr(src, fileName, err.Error())
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
		log.Debug(fmt.Sprintf("Copying '%s' from '%s' to '%s'", fileName, src, dstDirPath))
		if err = fileutils.CreateDirIfNotExist(dstDirPath); err != nil {
			return
		}
		if err = fileutils.CopyFile(dstDirPath, srcPath); err != nil {
			return
		}
	}
	return
}

// Send reload request to the Artifactory in order to reload the plugin files.
func (idtp *InstallDataTransferPluginCommand) sendReloadRequest() error {
	serviceManager, err := utils.CreateServiceManager(idtp.targetServer, -1, 0, false)
	if err != nil {
		return err
	}
	serviceDetails := serviceManager.GetConfig().GetServiceDetails()
	httpDetails := serviceDetails.CreateHttpClientDetails()

	resp, body, err := serviceManager.Client().SendPost(serviceDetails.GetUrl()+pluginReloadRestApi, []byte{}, &httpDetails)
	if err != nil {
		return err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return err
	}
	return nil
}

func (idtp *InstallDataTransferPluginCommand) Run() (err error) {
	log.Info(coreutils.PrintBoldTitle(fmt.Sprintf("Installing '%s' plugin...", pluginName)))

	// Get source, destination and transfer action
	dst, err := idtp.getPluginDirDestination()
	if err != nil {
		return
	}
	src, transferAction, err := idtp.getTransferSourceAndAction()
	if err != nil {
		return
	}
	// Execute transferring action
	if err = transferAction(src, dst.toPath(), idtp.transferManger.files); err != nil {
		return
	}
	// Reload plugins
	if err = idtp.sendReloadRequest(); err != nil {
		return
	}

	log.Info(coreutils.PrintBoldTitle(fmt.Sprintf("The %s plugin installed successfully.", pluginName)))
	return
}
