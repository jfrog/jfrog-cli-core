package transferconfig

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transfer"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferconfig/configxmlutils"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	minArtifactoryVersion               = "6.23.21"
	importStartRetries                  = 3
	importStartRetriesIntervalMilliSecs = 10000
	importPollingTimeout                = 10 * time.Minute
	importPollingInterval               = 10 * time.Second
)

type TransferConfigCommand struct {
	sourceServerDetails  *config.ServerDetails
	targetServerDetails  *config.ServerDetails
	dryRun               bool
	force                bool
	verbose              bool
	includeReposPatterns []string
	excludeReposPatterns []string
}

func NewTransferConfigCommand(sourceServer, targetServer *config.ServerDetails) *TransferConfigCommand {
	return &TransferConfigCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer}
}

func (tcc *TransferConfigCommand) CommandName() string {
	return "rt_transfer_config"
}

func (tcc *TransferConfigCommand) SetDryRun(dryRun bool) *TransferConfigCommand {
	tcc.dryRun = dryRun
	return tcc
}

func (tcc *TransferConfigCommand) SetForce(force bool) *TransferConfigCommand {
	tcc.force = force
	return tcc
}

func (tcc *TransferConfigCommand) SetVerbose(verbose bool) *TransferConfigCommand {
	tcc.verbose = verbose
	return tcc
}

func (tcc *TransferConfigCommand) SetIncludeReposPatterns(includeReposPatterns []string) *TransferConfigCommand {
	tcc.includeReposPatterns = includeReposPatterns
	return tcc
}

func (tcc *TransferConfigCommand) SetExcludeReposPatterns(excludeReposPatterns []string) *TransferConfigCommand {
	tcc.excludeReposPatterns = excludeReposPatterns
	return tcc
}

func (tcc *TransferConfigCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 1/4 - Preparations ==========")))
	// Create config managers
	sourceServicesManager, err := utils.CreateServiceManager(tcc.sourceServerDetails, -1, 0, tcc.dryRun)
	if err != nil {
		return
	}
	targetServiceManager, err := utils.CreateServiceManager(tcc.targetServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	log.Info("Verifying minimum version of the source server...")
	sourceArtifactoryVersion, err := sourceServicesManager.GetVersion()
	if err != nil {
		return
	}

	// Make sure that the source and target Artifactory servers are different and that the target Artifactory is empty
	if err = tcc.validateArtifactoryServers(targetServiceManager, sourceArtifactoryVersion); err != nil {
		return
	}

	// Run export on the source Artifactory
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 2/4 - Export configuration from the source Artifactory ==========")))
	exportPath, cleanUp, err := tcc.exportSourceArtifactory(sourceServicesManager)
	defer func() {
		cleanUpErr := cleanUp()
		if err == nil {
			err = cleanUpErr
		}
	}()
	if err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 3/4 - Download and modify configuration ==========")))

	// Download and decrypt the config XML from the source Artifactory
	configXml, err := tcc.getConfigXml(sourceServicesManager, sourceArtifactoryVersion)
	if err != nil {
		return
	}

	// Filter repositories to transfer
	transferRepositories, err := transfer.GetFilteredRepositories(sourceServicesManager, tcc.includeReposPatterns, tcc.excludeReposPatterns, false)
	if err != nil {
		return
	}

	// Prepare the config XML to be imported to SaaS
	configXml, err = tcc.modifyConfigXml(configXml, tcc.sourceServerDetails.ArtifactoryUrl, tcc.targetServerDetails.AccessUrl, transferRepositories)
	if err != nil {
		return
	}

	// Create an archive of the source Artifactory export in memory
	archiveConfig, err := archiveConfig(exportPath, configXml)
	if err != nil {
		return
	}

	// Import the archive to the target Artifactory
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 4/4 - Import configuration to the target Artifactory ==========")))
	err = tcc.importToTargetArtifactory(targetServiceManager, archiveConfig)
	if err != nil {
		return
	}

	// Update the server details of the target Artifactory in the CLI configuration
	return tcc.updateServerDetails()
}

// Make sure source and target Artifactory URLs are different.
// Make sure the target Artifactory is empty, by counting the number of the repositories. If it is bigger than 1, return an error.
// Also make sure that the source Artifactory version is sufficient.
func (tcc *TransferConfigCommand) validateArtifactoryServers(targetServicesManager artifactory.ArtifactoryServicesManager, sourceArtifactoryVersion string) error {
	if !version.NewVersion(sourceArtifactoryVersion).AtLeast(minArtifactoryVersion) {
		return errorutils.CheckErrorf("This operation requires source Artifactory version %s or higher", minArtifactoryVersion)
	}

	// Avoid exporting and importing to the same server
	log.Info("Verifying source and target servers are different...")
	if tcc.sourceServerDetails.GetArtifactoryUrl() == tcc.targetServerDetails.GetArtifactoryUrl() {
		return errorutils.CheckErrorf("The source and target Artifactory servers are identical, but should be different.")
	}

	// Verify installation of the config-import plugin in the target server and make sure that the user is admin
	log.Info("Verifying config-import plugin is installed in the target server...")
	if err := tcc.verifyConfigImportPlugin(targetServicesManager); err != nil {
		return err
	}

	if tcc.force {
		return nil
	}
	log.Info("Verifying target server is empty...")
	users, err := targetServicesManager.GetAllUsers()
	if err != nil {
		return err
	}
	// We consider an "empty" Artifactory as an Artifactory server that contains 2 users: the admin user and the anonymous.
	if len(users) > 2 {
		return errorutils.CheckErrorf("cowardly refusing to import the config to the target server, because it contains more than 2 users. By default, this command avoids transferring the config to a server which isn't empty. You can bypass this rule by providing the --force flag to the transfer-config command.")
	}
	return nil
}

func (tcc *TransferConfigCommand) verifyConfigImportPlugin(targetServicesManager artifactory.ArtifactoryServicesManager) error {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.targetServerDetails.GetArtifactoryUrl())

	// Create rtDetails
	rtDetails, err := createArtifactoryClientDetails(targetServicesManager)
	if err != nil {
		return err
	}

	// Get config-import plugin version
	configImportVersionUrl := artifactoryUrl + "api/plugins/execute/configImportVersion"
	configImportPluginVersion, err := commandsUtils.GetTransferPluginVersion(targetServicesManager.Client(), configImportVersionUrl, "config-import", rtDetails)
	if err != nil {
		return err
	}
	log.Info("config-import plugin version: " + configImportPluginVersion)

	// Execute 'GET /api/plugins/execute/checkPermissions'
	resp, _, _, err := targetServicesManager.Client().SendGet(artifactoryUrl+"api/plugins/execute/checkPermissions", false, rtDetails)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Unexpected status received: 403 if the user is not admin, 500+ if there is a server error
	errorBody, _ := io.ReadAll(resp.Body)
	messageFormat := fmt.Sprintf("Target server response: %s.\n%s", resp.Status, errorBody)
	return errorutils.CheckErrorf(messageFormat)
}

// Download and decrypt artifactory.config.xml from the source Artifactory server.
// It is safer to not store the decrypted artifactory.config.xml file in the file system, and therefore we only keep it in memory.
func (tcc *TransferConfigCommand) getConfigXml(sourceServiceManager artifactory.ArtifactoryServicesManager, sourceArtifactoryVersion string) (configXml string, err error) {
	// For Artifactory 6, in some cases, the artifactory.config.xml may not be decrypted and the following error returned:
	// 409: Cannot decrypt without artifactory key file
	if !version.NewVersion(sourceArtifactoryVersion).AtLeast("7.0.0") {
		if err = sourceServiceManager.ActivateKeyEncryption(); err != nil {
			return
		}
	}

	if err = sourceServiceManager.DeactivateKeyEncryption(); err != nil {
		return "", err
	}
	defer func() {
		activationErr := sourceServiceManager.ActivateKeyEncryption()
		if err == nil {
			err = activationErr
		}
	}()
	return sourceServiceManager.GetConfigDescriptor()
}

// Export the config from the source Artifactory to a local directory.
// Return the path to the export directory, a cleanup function and an error.
func (tcc *TransferConfigCommand) exportSourceArtifactory(sourceServicesManager artifactory.ArtifactoryServicesManager) (string, func() error, error) {
	// Create temp directory that will contain the export directory
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return "", func() error { return nil }, err
	}

	if err = os.Chmod(tempDir, 0777); err != nil {
		return "", func() error { return nil }, errorutils.CheckError(err)
	}

	// Do export
	trueValue := true
	falseValue := false
	exportParams := services.ExportParams{
		ExportPath:      tempDir,
		IncludeMetadata: &falseValue,
		Verbose:         &tcc.verbose,
		ExcludeContent:  &trueValue,
	}
	cleanUp := func() error { return fileutils.RemoveTempDir(tempDir) }
	if err = sourceServicesManager.Export(exportParams); err != nil {
		return "", cleanUp, err
	}

	// Make sure only the export directory contained in the temp directory
	files, err := fileutils.ListFiles(tempDir, true)
	if err != nil {
		return "", cleanUp, err
	}
	if len(files) == 0 {
		return "", cleanUp, errorutils.CheckErrorf("couldn't find the export directory in '%s'. Please make sure to run this command inside the source Artifactory machine", tempDir)
	}
	if len(files) > 1 {
		return "", cleanUp, errorutils.CheckErrorf("only the exported directory is expected to be in the export directory %s, but found %q", tempDir, files)
	}

	// Return the export directory and the cleanup function
	return files[0], cleanUp, nil
}

// Modify artifactory.config.xml:
// 1. Remove non-included repositories, if provided
// 2. Replace URL of federated repositories from sourceBaseUrl to targetBaseUrl
func (tcc *TransferConfigCommand) modifyConfigXml(configXml, sourceBaseUrl, targetBaseUrl string, transferRepositories []string) (string, error) {
	var err error
	if len(transferRepositories) > 0 {
		configXml, err = configxmlutils.RemoveNonIncludedRepositories(configXml, transferRepositories)
		if err != nil {
			return "", err
		}
	}
	return configxmlutils.ReplaceUrlsInFederatedrepos(configXml, sourceBaseUrl, targetBaseUrl)
}

// Import from the input buffer to the target Artifactory
func (tcc *TransferConfigCommand) importToTargetArtifactory(targetServicesManager artifactory.ArtifactoryServicesManager, buffer *bytes.Buffer) (err error) {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.targetServerDetails.GetArtifactoryUrl())
	var timestamp []byte

	// Create rtDetails
	rtDetails, err := createArtifactoryClientDetails(targetServicesManager)
	if err != nil {
		return err
	}

	// Sometimes, POST api/plugins/execute/configImport return unexpectedly 404 errors, altough the config-import plugin is installed.
	// To overcome this issue, we use a custom retryExecutor and not the default retry executor that retries only on HTTP errors >= 500.
	retryExecutor := clientutils.RetryExecutor{
		MaxRetries:               importStartRetries,
		RetriesIntervalMilliSecs: importStartRetriesIntervalMilliSecs,
		ErrorMessage:             fmt.Sprintf("Failed to start the config import process in %s", artifactoryUrl),
		LogMsgPrefix:             "[Config import]",
		ExecutionHandler: func() (shouldRetry bool, err error) {
			// Start the config import async process
			resp, body, err := targetServicesManager.Client().SendPost(artifactoryUrl+"api/plugins/execute/configImport", buffer.Bytes(), rtDetails)
			if err != nil {
				return false, err
			}
			if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
				return true, errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
			}

			log.Debug("Artifactory response: ", resp.Status)
			timestamp = body
			log.Info("Config import timestamp: " + string(timestamp))
			return false, nil
		},
	}

	if err = retryExecutor.Execute(); err != nil {
		return err
	}

	// Wait for config import completion
	return tcc.waitForImportCompletion(targetServicesManager, rtDetails, timestamp)
}

func (tcc *TransferConfigCommand) waitForImportCompletion(targetServicesManager artifactory.ArtifactoryServicesManager, rtDetails *httputils.HttpClientDetails, importTimestamp []byte) error {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.targetServerDetails.GetArtifactoryUrl())

	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         importPollingTimeout,
		PollingInterval: importPollingInterval,
		MsgPrefix:       "Waiting for config import completion in Artifactory server at " + artifactoryUrl,
		PollingAction:   tcc.createImportPollingAction(targetServicesManager, rtDetails, artifactoryUrl, importTimestamp),
	}

	body, err := pollingExecutor.Execute()
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Logs from Artifactory:\n%s", body))
	return nil
}

func (tcc *TransferConfigCommand) createImportPollingAction(targetServicesManager artifactory.ArtifactoryServicesManager, rtDetails *httputils.HttpClientDetails, artifactoryUrl string, importTimestamp []byte) httputils.PollingAction {
	return func() (shouldStop bool, responseBody []byte, err error) {
		// Get config import status
		resp, body, err := targetServicesManager.Client().SendPost(artifactoryUrl+"api/plugins/execute/configImportStatus", importTimestamp, rtDetails)
		if err != nil {
			return true, nil, err
		}

		// 200 - Import completed
		if resp.StatusCode == http.StatusOK {
			return true, body, nil
		}

		// 202 - Import in progress
		if resp.StatusCode == http.StatusAccepted {
			return false, nil, nil
		}

		// Unexpected status
		if err = errorutils.CheckResponseStatus(resp, http.StatusUnauthorized, http.StatusForbidden); err != nil {
			return false, nil, errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
		}

		// 401 or 403 - The user used for the target Artifactory server does not exist anymore.
		// This is perfectly normal, because the import caused the user to be deleted. We can now use the credentials of the source Artifactory server.
		newServerDetails := tcc.targetServerDetails
		newServerDetails.SetUser(tcc.sourceServerDetails.GetUser())
		newServerDetails.SetPassword(tcc.sourceServerDetails.GetPassword())
		newServerDetails.SetAccessToken(tcc.sourceServerDetails.GetAccessToken())

		targetServicesManager, err = utils.CreateServiceManager(newServerDetails, -1, 0, false)
		if err != nil {
			return true, nil, err
		}
		rtDetails, err = createArtifactoryClientDetails(targetServicesManager)
		if err != nil {
			return true, nil, err
		}

		// After 401 or 403, the server credentials are fixed, and therefore we can run again
		return false, nil, nil
	}
}

func (tcc *TransferConfigCommand) updateServerDetails() error {
	log.Info("Pinging the target Artifactory...")
	newTargetServerDetails := tcc.targetServerDetails

	// Copy credentials from the source server details
	newTargetServerDetails.User = tcc.sourceServerDetails.User
	newTargetServerDetails.Password = tcc.sourceServerDetails.Password
	newTargetServerDetails.SshKeyPath = tcc.sourceServerDetails.SshKeyPath
	newTargetServerDetails.SshPassphrase = tcc.sourceServerDetails.SshPassphrase
	newTargetServerDetails.AccessToken = tcc.sourceServerDetails.AccessToken
	newTargetServerDetails.RefreshToken = tcc.sourceServerDetails.RefreshToken
	newTargetServerDetails.ArtifactoryRefreshToken = tcc.sourceServerDetails.ArtifactoryRefreshToken
	newTargetServerDetails.ArtifactoryTokenRefreshInterval = tcc.sourceServerDetails.ArtifactoryTokenRefreshInterval
	newTargetServerDetails.ClientCertPath = tcc.sourceServerDetails.ClientCertPath
	newTargetServerDetails.ClientCertKeyPath = tcc.sourceServerDetails.ClientCertKeyPath

	// Ping to validate the transfer ended successfully
	pingCmd := generic.NewPingCommand().SetServerDetails(newTargetServerDetails)
	err := pingCmd.Run()
	if err != nil {
		return err
	}
	log.Info("Ping to the target Artifactory was successful. Updating the server configuration in JFrog CLI.")

	// Update the server details in JFrog CLI configuration
	configCmd := commands.NewConfigCommand(commands.AddOrEdit, newTargetServerDetails.ServerId).SetInteractive(false).SetDetails(newTargetServerDetails)
	err = configCmd.Run()
	if err != nil {
		return err
	}
	tcc.targetServerDetails = newTargetServerDetails
	return nil
}
