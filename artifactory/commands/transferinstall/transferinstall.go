package transferinstall

import (
	"fmt"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"path"
)

const (
	minArtifactoryVersion = "2.9.0" // for reload
	groovyFileName        = "dataTransfer.groovy"
	jarFileName           = "data-transfer.jar"
	dataTransferUrl       = "https://releases.jfrog.io/artifactory/jfrog-releases/data-transfer"
	jHomeEnvVar           = "JFROG_HOME"
	downloadRetries       = 3
	latest                = "[RELEASE]"
	pluginReloadRestApi   = "api/plugins/reload"
)

var (
	pluginDir        = path.Join("etc", "plugins")
	pluginDirV7Above = path.Join("var", "etc", "artifactory", "plugins")
)

type TransferInstallCommand struct {
	// The server that the plugin will be installed on
	targetServer *config.ServerDetails
	// The version we want to download from web
	installVersion *version.Version
	// The information about the local file paths for the option to provide the files
	srcPluginFiles *TransferPluginFiles
}

func NewTransferInstallCommand(server *config.ServerDetails) *TransferInstallCommand {
	return &TransferInstallCommand{
		targetServer:   server,
		srcPluginFiles: nil,
		installVersion: nil, // latest
	}
}

// TransferPluginFiles represent all the plugin files and their paths as value
type TransferPluginFiles struct {
	groovyFile string
	jarFile    string
}

type installTransferAction func(dst, src string) error

func (tpf *TransferPluginFiles) transfer(dst *TransferPluginFiles, action installTransferAction) error {
	// Groovy file
	err := action(dst.groovyFile, tpf.groovyFile)
	if err != nil {
		return err
	}
	// Jar file
	err = action(dst.jarFile, tpf.jarFile)
	if err != nil {
		return err
	}
	return nil
}

// Create a representation of the plugin files as if all of them are at the same folder
func NewSrcTransferPluginFiles(directory string) *TransferPluginFiles {
	return &TransferPluginFiles{
		groovyFile: path.Join(directory, groovyFileName),
		jarFile:    path.Join(directory, jarFileName),
	}
}

// Create the destination directory representation for each file base on the plugin dir path
func getDstPluginPaths(pluginDir string, asUrl bool) *TransferPluginFiles {
	if asUrl {
		return &TransferPluginFiles{
			groovyFile: pluginDir + "/" + groovyFileName,
			jarFile:    pluginDir + "/lib/" + jarFileName,
		}
	} else {
		return &TransferPluginFiles{
			groovyFile: path.Join(pluginDir, groovyFileName),
			jarFile:    path.Join(pluginDir, "lib", jarFileName),
		}
	}
}

func (tic *TransferInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return tic.targetServer, nil
}

func (tic *TransferInstallCommand) CommandName() string {
	return "rt_transfer_install"
}

func (tic *TransferInstallCommand) SetLocalPluginFiles(pluginFiles *TransferPluginFiles) *TransferInstallCommand {
	tic.srcPluginFiles = pluginFiles
	return tic
}

func (tic *TransferInstallCommand) SetInstallVersion(installVersion *version.Version) *TransferInstallCommand {
	tic.installVersion = installVersion
	return tic
}

// Search the target directory that the plugins exists in base on a given root JFrog home directory
// we support the all the directory structures, first trying the latest structure and fallback to old if not found
func (tic *TransferInstallCommand) getTargetInstallDirectory(homePath string) (targetPath string, err error) {
	targetPath = path.Join(homePath, "artifactory")
	// Search artifactory directory structure V7 +
	exists, err := fileutils.IsDirExists(path.Join(targetPath, pluginDirV7Above), false)
	if err != nil {
		return
	}
	if exists {
		targetPath = path.Join(targetPath, pluginDirV7Above)
		return
	}
	// Search artifactory directory structure up to and include V6
	exists, err = fileutils.IsDirExists(path.Join(targetPath, pluginDir), false)
	if err != nil {
		return
	}
	if exists {
		targetPath = path.Join(targetPath, pluginDir)
		return
	}
	err = errors.Errorf("path '%s' is not a valid directory.", homePath)
	return
}

// Returns the plugin download url string base on the version we want to download, if nil it will return the latest
func parseVersionToDownloadUrl(version *version.Version) string {
	if version == nil {
		return dataTransferUrl + "/" + latest
	} else {
		return dataTransferUrl + "/" + version.GetVersion()
	}
}

// Send a get request to a given url and get its body if request status is OK
func fetchContentFromWeb(url string) (body []byte, err error) {
	client, err := httpclient.ClientBuilder().SetRetries(downloadRetries).Build()
	if err != nil {
		return
	}
	resp, body, _, err := client.SendGet(url, true, httputils.HttpClientDetails{Headers: make(map[string]string)}, "")
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return
	}
	return
}

// fetch content from web base on src url and writes it to a file base on dst full path (dir + name)
// it also creates the directory if it does not exist
func downloadFile(dst, src string) (err error) {
	// Get content from server
	content, err := fetchContentFromWeb(src)
	if err != nil {
		return
	}
	// Save to directory
	_, dir := fileutils.GetFileAndDirFromPath(dst)
	if err = fileutils.CreateDirIfNotExist(dir); err != nil {
		return
	}
	return os.WriteFile(dst, content, 0777)
}

// Copy the transfer plugin files from the given source to the target directory (create path if needed or override existing files)
func (tic *TransferInstallCommand) copyPluginFilesToTarget(source *TransferPluginFiles, targetDir string) error {
	dst := getDstPluginPaths(targetDir, false)
	return source.transfer(dst, fileutils.CopyFile)
}

// Download the transfer plugin files from the given url to the target directory (create path if needed or override existing files)
func (tic *TransferInstallCommand) downloadPluginFilesToTarget(mainUrl, targetDir string) error {
	dst := getDstPluginPaths(targetDir, false)
	urls := getDstPluginPaths(mainUrl, true)
	return urls.transfer(dst, downloadFile)
}

// Send reload command to the artifactory in order to reload the plugin files
func (tic *TransferInstallCommand) runReLoadCommand(servicesManager artifactory.ArtifactoryServicesManager) error {
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

// Validates the information needed for this command: minimum version (for reload API request), JFROG_HOME env var exists
func (tic *TransferInstallCommand) validateServer(servicesManager artifactory.ArtifactoryServicesManager) error {
	// validate version
	artifactoryVersion, err := servicesManager.GetVersion()
	if err != nil {
		return err
	}
	if !version.NewVersion(artifactoryVersion).AtLeast(minArtifactoryVersion) {
		return errorutils.CheckErrorf("This operation requires Artifactory version %s or higher", minArtifactoryVersion)
	}
	// validate environment variable
	_, exists := os.LookupEnv(jHomeEnvVar)
	if !exists {
		return errors.Errorf("The environment variable '" + jHomeEnvVar + "' must be defined.")
	}
	return nil
}

func (tic *TransferInstallCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("Installing transfer plugin...")))

	// Phase 0: initialize and validate
	serviceManager, err := utils.CreateServiceManager(tic.targetServer, -1, 0, false)
	if err != nil {
		return
	}
	log.Debug("Verifying environment and minimum version of the server...")
	if err = tic.validateServer(serviceManager); err != nil {
		return
	}
	jHomePath := os.Getenv(jHomeEnvVar)

	// Phase 1: Get the correct target directory we will install on
	log.Debug(fmt.Sprintf("searching inside '%s' for plugin directory", jHomePath))
	installTargetDir, err := tic.getTargetInstallDirectory(jHomePath)
	if err != nil {
		return
	}
	log.Debug(fmt.Sprintf("found target plugin directory at '%s'", installTargetDir))

	// Phase 2: fetch the files we need to install and make a copy in the target directory
	if tic.srcPluginFiles != nil {
		log.Debug("local plugin files provided. copying from file system")
		err = tic.copyPluginFilesToTarget(tic.srcPluginFiles, installTargetDir)
	} else {
		// no local files provided, we need to fetch them from web
		if tic.installVersion == nil {
			log.Debug("fetching latest version to the target.")
		} else {
			log.Debug(fmt.Sprintf("fetching transfer plugin version '%s' to the target.", tic.installVersion.GetVersion()))
		}
		err = tic.downloadPluginFilesToTarget(parseVersionToDownloadUrl(tic.installVersion), installTargetDir)
	}
	if err != nil {
		return
	}

	// Phase 3: Reload
	log.Debug("plugin files fetched to the target directory, reloading plugins")
	err = tic.runReLoadCommand(serviceManager)
	if err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("Transfer plugin was installed successfully!.")))
	return nil
}
