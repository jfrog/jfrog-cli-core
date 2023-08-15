package transferconfig

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jfrog/gofrog/version"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferconfig/configxmlutils"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	importStartRetries                  = 3
	importStartRetriesIntervalMilliSecs = 10000
	importPollingTimeout                = 10 * time.Minute
	importPollingInterval               = 10 * time.Second
	interruptedByUserErr                = "Config transfer was cancelled"
	minTransferConfigArtifactoryVersion = "6.23.21"
)

type TransferConfigCommand struct {
	commandsUtils.TransferConfigBase
	dryRun           bool
	force            bool
	verbose          bool
	preChecks        bool
	sourceWorkingDir string
	targetWorkingDir string
}

func NewTransferConfigCommand(sourceServer, targetServer *config.ServerDetails) *TransferConfigCommand {
	return &TransferConfigCommand{TransferConfigBase: *commandsUtils.NewTransferConfigBase(sourceServer, targetServer)}
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

func (tcc *TransferConfigCommand) SetPreChecks(preChecks bool) *TransferConfigCommand {
	tcc.preChecks = preChecks
	return tcc
}

func (tcc *TransferConfigCommand) SetSourceWorkingDir(workingDir string) *TransferConfigCommand {
	tcc.sourceWorkingDir = workingDir
	return tcc
}

func (tcc *TransferConfigCommand) SetTargetWorkingDir(workingDir string) *TransferConfigCommand {
	tcc.targetWorkingDir = workingDir
	return tcc
}

func (tcc *TransferConfigCommand) Run() (err error) {
	if err = tcc.CreateServiceManagers(tcc.dryRun); err != nil {
		return err
	}
	if tcc.preChecks {
		return tcc.runPreChecks()
	}

	tcc.LogTitle("Phase 1/5 - Preparations")
	err = tcc.printWarnings()
	if err != nil {
		return err
	}
	err = tcc.validateServerPrerequisites()
	if err != nil {
		return err
	}
	// Run export on the source Artifactory
	tcc.LogTitle("Phase 2/5 - Export configuration from the source Artifactory")
	exportPath, cleanUp, err := tcc.exportSourceArtifactory()
	defer func() {
		cleanUpErr := cleanUp()
		if err == nil {
			err = cleanUpErr
		}
	}()
	if err != nil {
		return
	}

	tcc.LogTitle("Phase 3/5 - Download and modify configuration")
	selectedRepos, err := tcc.GetSelectedRepositories()
	if err != nil {
		return
	}

	// Download and decrypt the config XML from the source Artifactory
	configXml, remoteRepos, err := tcc.getEncryptedItems(selectedRepos)
	if err != nil {
		return
	}

	// In Artifactory 7.49, the repositories configuration was moved from the artifactory-config.xml to the database.
	// this method removes the repositories from the artifactory-config.xml file, to be aligned with new Artifactory versions.
	configXml, err = configxmlutils.RemoveAllRepositories(configXml)
	if err != nil {
		return
	}

	// Create an archive of the source Artifactory export in memory
	archiveConfig, err := archiveConfig(exportPath, configXml)
	if err != nil {
		return
	}

	// Import the archive to the target Artifactory
	tcc.LogTitle("Phase 4/5 - Import configuration to the target Artifactory")
	err = tcc.importToTargetArtifactory(archiveConfig)
	if err != nil {
		return
	}

	// Update the server details of the target Artifactory in the CLI configuration
	err = tcc.updateServerDetails()
	if err != nil {
		return
	}

	tcc.LogTitle("Phase 5/5 - Import repositories to the target Artifactory")
	if err = tcc.TransferRepositoriesToTarget(selectedRepos, remoteRepos); err != nil {
		return
	}

	// If config transfer passed successfully, add conclusion message
	log.Output()
	log.Info("Config transfer completed successfully!")
	tcc.LogIfFederatedMemberRemoved()
	log.Info("☝️  Please make sure to disable configuration transfer in MyJFrog before running the 'jf transfer-files' command.")
	return
}

// Create the directory containing the Artifactory export content
// Return values:
// exportPath - The export path
// unsetTempDir - Clean up function
// err - Error if any
func (tcc *TransferConfigCommand) createExportPath() (exportPath string, unsetTempDir func(), err error) {
	if tcc.sourceWorkingDir != "" {
		// Set the base temp dir according to the value of the --source-working-dir flag
		oldTempDir := fileutils.GetTempDirBase()
		fileutils.SetTempDirBase(tcc.sourceWorkingDir)
		unsetTempDir = func() {
			fileutils.SetTempDirBase(oldTempDir)
		}
	} else {
		unsetTempDir = func() {}
	}

	// Create temp directory that will contain the export directory
	exportPath, err = fileutils.CreateTempDir()
	if err != nil {
		return "", unsetTempDir, err
	}

	return exportPath, unsetTempDir, errorutils.CheckError(os.Chmod(exportPath, 0777))
}

func (tcc *TransferConfigCommand) runPreChecks() error {
	// Warn if default admin:password credentials are exist in the source server
	_, err := tcc.IsDefaultCredentials()
	if err != nil {
		return err
	}

	if err = tcc.validateServerPrerequisites(); err != nil {
		return err
	}

	selectedRepos, err := tcc.GetSelectedRepositories()
	if err != nil {
		return err
	}

	// Download and decrypt the remote repositories list from the source Artifactory
	_, remoteRepositories, err := tcc.getEncryptedItems(selectedRepos)
	if err != nil {
		return err
	}

	return tcc.NewPreChecksRunner(remoteRepositories).Run(context.Background(), tcc.TargetServerDetails)
}

func (tcc *TransferConfigCommand) printWarnings() (err error) {
	// Prompt message
	promptMsg := "This command will transfer Artifactory config data:\n" +
		fmt.Sprintf("From %s - <%s>\n", coreutils.PrintBold("Source"), tcc.SourceServerDetails.ArtifactoryUrl) +
		fmt.Sprintf("To %s - <%s>\n", coreutils.PrintBold("Target"), tcc.TargetServerDetails.ArtifactoryUrl) +
		"This action will wipe out all Artifactory content in the target.\n" +
		"Make sure that you're using strong credentials in your source platform (for example - having the default admin:password credentials isn't recommended).\n" +
		"Those credentials will be transferred to your SaaS target platform.\n" +
		"Are you sure you want to continue?"

	if !coreutils.AskYesNo(promptMsg, false) {
		return errorutils.CheckErrorf(interruptedByUserErr)
	}

	// Check if there is a configured user using default credentials in the source platform.
	isDefaultCredentials, err := tcc.IsDefaultCredentials()
	if err != nil {
		return err
	}
	if isDefaultCredentials && !coreutils.AskYesNo("Are you sure you want to continue?", false) {
		return errorutils.CheckErrorf(interruptedByUserErr)
	}
	return nil
}

// Make sure the target Artifactory is empty, by counting the number of the users. If it is bigger than 1, return an error.
// Also, make sure that the config-import plugin is installed
func (tcc *TransferConfigCommand) validateTargetServer() error {
	// Verify installation of the config-import plugin in the target server and make sure that the user is admin
	log.Info("Verifying config-import plugin is installed in the target server...")
	if err := tcc.verifyConfigImportPlugin(); err != nil {
		return err
	}

	if tcc.force {
		return nil
	}
	log.Info("Verifying target server is empty...")
	users, err := tcc.TargetArtifactoryManager.GetAllUsers()
	if err != nil {
		return err
	}
	// We consider an "empty" Artifactory as an Artifactory server that contains 2 users: the admin user and the anonymous.
	if len(users) > 2 {
		return errorutils.CheckErrorf("cowardly refusing to import the config to the target server, because it contains more than 2 users. By default, this command avoids transferring the config to a server which isn't empty. You can bypass this rule by providing the --force flag to the transfer-config command.")
	}
	return nil
}

func (tcc *TransferConfigCommand) verifyConfigImportPlugin() error {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.TargetServerDetails.GetArtifactoryUrl())

	// Create rtDetails
	rtDetails, err := commandsUtils.CreateArtifactoryClientDetails(tcc.TargetArtifactoryManager)
	if err != nil {
		return err
	}

	// Get config-import plugin version
	configImportVersionUrl := artifactoryUrl + commandsUtils.PluginsExecuteRestApi + "configImportVersion"
	configImportPluginVersion, err := commandsUtils.GetTransferPluginVersion(tcc.TargetArtifactoryManager.Client(), configImportVersionUrl, "config-import", commandsUtils.Target, rtDetails)
	if err != nil {
		return err
	}
	log.Info("config-import plugin version: " + configImportPluginVersion)

	// Execute 'GET /api/plugins/execute/checkPermissions'
	resp, body, _, err := tcc.TargetArtifactoryManager.Client().SendGet(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"checkPermissions"+tcc.getWorkingDirParam(), false, rtDetails)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Unexpected status received: 403 if the user is not admin, 500+ if there is a server error
	messageFormat := fmt.Sprintf("Target server response: %s.\n%s", resp.Status, body)
	return errorutils.CheckErrorf(messageFormat)
}

// Creates the Pre-checks runner for the config import command
func (tcc *TransferConfigCommand) NewPreChecksRunner(remoteRepositories []interface{}) (runner *commandsUtils.PreCheckRunner) {
	runner = commandsUtils.NewPreChecksRunner()

	// Add pre-checks here
	runner.AddCheck(commandsUtils.NewRemoteRepositoryCheck(&tcc.TargetArtifactoryManager, remoteRepositories))

	return
}

func (tcc *TransferConfigCommand) getEncryptedItems(selectedSourceRepos map[utils.RepoType][]string) (configXml string, remoteRepositories []interface{}, err error) {
	reactivateKeyEncryption, err := tcc.DeactivateKeyEncryption()
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if reactivationErr := reactivateKeyEncryption(); err == nil {
			err = reactivationErr
		}
	}()

	// Download artifactory.config.xml from the source Artifactory server.
	// It is safer to not store the decrypted artifactory.config.xml file in the file system, and therefore we only keep it in memory.
	configXml, err = tcc.SourceArtifactoryManager.GetConfigDescriptor()
	if err != nil {
		return
	}

	// Get all remote repositories from the source Artifactory server.
	if remoteRepositoryNames, ok := selectedSourceRepos[utils.Remote]; ok && len(remoteRepositoryNames) > 0 {
		remoteRepositories = make([]interface{}, len(remoteRepositoryNames))
		for i, repoName := range remoteRepositoryNames {
			if err = tcc.SourceArtifactoryManager.GetRepository(repoName, &remoteRepositories[i]); err != nil {
				return
			}
		}
	}

	return
}

// Export the config from the source Artifactory to a local directory.
// Return the path to the export directory, a cleanup function and an error.
func (tcc *TransferConfigCommand) exportSourceArtifactory() (string, func() error, error) {
	// Create temp directory that will contain the export directory
	exportPath, unsetTempDir, err := tcc.createExportPath()
	defer unsetTempDir()
	if err != nil {
		return "", func() error { return nil }, err
	}

	// Do export
	trueValue := true
	falseValue := false
	exportParams := services.ExportParams{
		ExportPath:      exportPath,
		IncludeMetadata: &falseValue,
		Verbose:         &tcc.verbose,
		ExcludeContent:  &trueValue,
	}
	cleanUp := func() error { return fileutils.RemoveTempDir(exportPath) }
	if err = tcc.SourceArtifactoryManager.Export(exportParams); err != nil {
		return "", cleanUp, err
	}

	// Make sure only the export directory contained in the temp directory
	files, err := fileutils.ListFiles(exportPath, true)
	if err != nil {
		return "", cleanUp, err
	}
	if len(files) == 0 {
		return "", cleanUp, errorutils.CheckErrorf("couldn't find the export directory in '%s'. Please make sure to run this command inside the source Artifactory machine", exportPath)
	}
	if len(files) > 1 {
		return "", cleanUp, errorutils.CheckErrorf("only the exported directory is expected to be in the export directory %s, but found %q", exportPath, files)
	}

	// Return the export directory and the cleanup function
	return files[0], cleanUp, nil
}

// Import from the input buffer to the target Artifactory
func (tcc *TransferConfigCommand) importToTargetArtifactory(buffer *bytes.Buffer) (err error) {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.TargetServerDetails.GetArtifactoryUrl())
	var timestamp []byte

	// Create rtDetails
	rtDetails, err := commandsUtils.CreateArtifactoryClientDetails(tcc.TargetArtifactoryManager)
	if err != nil {
		return err
	}

	// Sometimes, POST api/plugins/execute/configImport return unexpectedly 404 errors, although the config-import plugin is installed.
	// To overcome this issue, we use a custom retryExecutor and not the default retry executor that retries only on HTTP errors >= 500.
	retryExecutor := clientutils.RetryExecutor{
		MaxRetries:               importStartRetries,
		RetriesIntervalMilliSecs: importStartRetriesIntervalMilliSecs,
		ErrorMessage:             fmt.Sprintf("Failed to start the config import process in %s", artifactoryUrl),
		LogMsgPrefix:             "[Config import]",
		ExecutionHandler: func() (shouldRetry bool, err error) {
			// Start the config import async process
			resp, body, err := tcc.TargetArtifactoryManager.Client().SendPost(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"configImport"+tcc.getWorkingDirParam(), buffer.Bytes(), rtDetails)
			if err != nil {
				return false, err
			}
			if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
				return true, err
			}

			log.Debug("Artifactory response:", resp.Status)
			timestamp = body
			log.Info("Config import timestamp: " + string(timestamp))
			return false, nil
		},
	}

	if err = retryExecutor.Execute(); err != nil {
		return err
	}

	// Wait for config import completion
	return tcc.waitForImportCompletion(rtDetails, timestamp)
}

func (tcc *TransferConfigCommand) waitForImportCompletion(rtDetails *httputils.HttpClientDetails, importTimestamp []byte) error {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.TargetServerDetails.GetArtifactoryUrl())

	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         importPollingTimeout,
		PollingInterval: importPollingInterval,
		MsgPrefix:       "Waiting for config import completion in Artifactory server at " + artifactoryUrl,
		PollingAction:   tcc.createImportPollingAction(rtDetails, artifactoryUrl, importTimestamp),
	}

	body, err := pollingExecutor.Execute()
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Logs from Artifactory:\n%s", body))
	if strings.Contains(string(body), "[ERROR]") {
		return errorutils.CheckErrorf("Errors detected during config import. Hint: You can skip transferring some Artifactory repositories by using the '--exclude-repos' command option. Run 'jf rt transfer-config -h' for more information.")
	}
	return nil
}

func (tcc *TransferConfigCommand) createImportPollingAction(rtDetails *httputils.HttpClientDetails, artifactoryUrl string, importTimestamp []byte) httputils.PollingAction {
	return func() (shouldStop bool, responseBody []byte, err error) {
		// Get config import status
		resp, body, err := tcc.TargetArtifactoryManager.Client().SendPost(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"configImportStatus"+tcc.getWorkingDirParam(), importTimestamp, rtDetails)
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
		if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusUnauthorized, http.StatusForbidden); err != nil {
			return false, nil, err
		}

		// 401 or 403 - The user used for the target Artifactory server does not exist anymore.
		// This is perfectly normal, because the import caused the user to be deleted. We can now use the credentials of the source Artifactory server.
		newServerDetails := tcc.TargetServerDetails
		newServerDetails.SetUser(tcc.SourceServerDetails.GetUser())
		newServerDetails.SetPassword(tcc.SourceServerDetails.GetPassword())
		newServerDetails.SetAccessToken(tcc.SourceServerDetails.GetAccessToken())

		tcc.TargetArtifactoryManager, err = utils.CreateServiceManager(newServerDetails, -1, 0, false)
		if err != nil {
			return true, nil, err
		}
		rtDetails, err = commandsUtils.CreateArtifactoryClientDetails(tcc.TargetArtifactoryManager)
		if err != nil {
			return true, nil, err
		}

		// After 401 or 403, the server credentials are fixed, and therefore we can run again
		return false, nil, nil
	}
}

func (tcc *TransferConfigCommand) updateServerDetails() error {
	log.Info("Pinging the target Artifactory...")
	newTargetServerDetails := tcc.TargetServerDetails

	// Copy credentials from the source server details
	newTargetServerDetails.User = tcc.SourceServerDetails.User
	newTargetServerDetails.Password = tcc.SourceServerDetails.Password
	newTargetServerDetails.SshKeyPath = tcc.SourceServerDetails.SshKeyPath
	newTargetServerDetails.SshPassphrase = tcc.SourceServerDetails.SshPassphrase
	newTargetServerDetails.AccessToken = tcc.SourceServerDetails.AccessToken
	newTargetServerDetails.RefreshToken = tcc.SourceServerDetails.RefreshToken
	newTargetServerDetails.ArtifactoryRefreshToken = tcc.SourceServerDetails.ArtifactoryRefreshToken
	newTargetServerDetails.ArtifactoryTokenRefreshInterval = tcc.SourceServerDetails.ArtifactoryTokenRefreshInterval
	newTargetServerDetails.ClientCertPath = tcc.SourceServerDetails.ClientCertPath
	newTargetServerDetails.ClientCertKeyPath = tcc.SourceServerDetails.ClientCertKeyPath

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
	tcc.TargetServerDetails = newTargetServerDetails
	return nil
}

func (tcc *TransferConfigCommand) getWorkingDirParam() string {
	if tcc.targetWorkingDir != "" {
		return "?params=workingDir=" + tcc.targetWorkingDir
	}
	return ""
}

// Make sure that the source Artifactory version is sufficient.
// Returns the source Artifactory version.
func (tcc *TransferConfigCommand) validateMinVersion() error {
	log.Info("Verifying minimum version of the source server...")
	sourceArtifactoryVersion, err := tcc.SourceArtifactoryManager.GetVersion()
	if err != nil {
		return err
	}
	targetArtifactoryVersion, err := tcc.TargetArtifactoryManager.GetVersion()
	if err != nil {
		return err
	}

	// Validate minimal Artifactory version in the source server
	err = coreutils.ValidateMinimumVersion(coreutils.Artifactory, sourceArtifactoryVersion, minTransferConfigArtifactoryVersion)
	if err != nil {
		return err
	}

	// Validate that the target Artifactory server version is >= than the source Artifactory server version
	if !version.NewVersion(targetArtifactoryVersion).AtLeast(sourceArtifactoryVersion) {
		return errorutils.CheckErrorf("The source Artifactory version (%s) can't be higher than the target Artifactory version (%s).", sourceArtifactoryVersion, targetArtifactoryVersion)
	}

	return nil
}

func (tcc *TransferConfigCommand) validateServerPrerequisites() error {
	// Make sure that the source Artifactory version is sufficient.
	if err := tcc.validateMinVersion(); err != nil {
		return err
	}
	// Make sure source and target Artifactory URLs are different
	if err := tcc.ValidateDifferentServers(); err != nil {
		return err
	}
	// Make sure that the target Artifactory is empty and the config-import plugin is installed
	return tcc.validateTargetServer()
}
