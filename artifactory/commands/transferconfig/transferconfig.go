package transferconfig

import (
	"bytes"
	"context"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
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
	"net/http"
	"os"
	"strings"
	"time"
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
	preChecks            bool
	includeReposPatterns []string
	excludeReposPatterns []string
	workingDir           string
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

func (tcc *TransferConfigCommand) SetPreChecks(preChecks bool) *TransferConfigCommand {
	tcc.preChecks = preChecks
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

func (tcc *TransferConfigCommand) SetWorkingDir(workingDir string) *TransferConfigCommand {
	tcc.workingDir = workingDir
	return tcc
}

func (tcc *TransferConfigCommand) getRepoFilter() *utils.IncludeExcludeFilter {
	return &utils.IncludeExcludeFilter{
		IncludePatterns: tcc.includeReposPatterns,
		ExcludePatterns: tcc.excludeReposPatterns,
	}
}

func (tcc *TransferConfigCommand) Run() (err error) {
	sourceServiceManager, err := utils.CreateServiceManager(tcc.sourceServerDetails, -1, 0, tcc.dryRun)
	if err != nil {
		return
	}
	targetServiceManager, err := utils.CreateServiceManager(tcc.targetServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	if tcc.preChecks {
		return tcc.runPreChecks(sourceServiceManager, targetServiceManager)
	}
	continueTransfer, err := tcc.printWarnings(sourceServiceManager)
	if err != nil || !continueTransfer {
		return
	}

	log.Info(coreutils.PrintBoldTitle("========== Phase 1/4 - Preparations =========="))
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	if _, err = validateMinVersionAndDifferentServers(sourceServiceManager, tcc.sourceServerDetails, tcc.targetServerDetails); err != nil {
		return
	}
	// Make sure that the target Artifactory is empty and the config-import plugin is installed
	if err = tcc.validateTargetServer(targetServiceManager); err != nil {
		return
	}

	// Run export on the source Artifactory
	log.Info(coreutils.PrintBoldTitle("========== Phase 2/4 - Export configuration from the source Artifactory =========="))
	exportPath, cleanUp, err := tcc.exportSourceArtifactory(sourceServiceManager)
	defer func() {
		cleanUpErr := cleanUp()
		if err == nil {
			err = cleanUpErr
		}
	}()
	if err != nil {
		return
	}

	log.Info(coreutils.PrintBoldTitle("========== Phase 3/4 - Download and modify configuration =========="))

	// Download and decrypt the config XML from the source Artifactory
	configXml, err := tcc.getConfigXml(sourceServiceManager)
	if err != nil {
		return
	}

	// Prepare the config XML to be imported to SaaS
	configXml, federatedMembersRemoved, err := tcc.modifyConfigXml(configXml)
	if err != nil {
		return
	}

	// Create an archive of the source Artifactory export in memory
	archiveConfig, err := archiveConfig(exportPath, configXml)
	if err != nil {
		return
	}

	// Import the archive to the target Artifactory
	log.Info(coreutils.PrintBoldTitle("========== Phase 4/4 - Import configuration to the target Artifactory =========="))
	err = tcc.importToTargetArtifactory(targetServiceManager, archiveConfig)
	if err != nil {
		return
	}

	// Update the server details of the target Artifactory in the CLI configuration
	err = tcc.updateServerDetails()
	if err != nil {
		return
	}

	// If config transfer passed successfully, add conclusion message
	log.Info("Config transfer completed successfully!")
	if federatedMembersRemoved {
		log.Info("☝️  Your Federated repositories have been transferred to your target instance, but their members have been removed on the target. " +
			"You should add members to your Federated repositories on your target instance as described here - https://www.jfrog.com/confluence/display/JFROG/Federated+Repositories.")
	}
	log.Info("☝️  Please make sure to disable configuration transfer in MyJFrog before running the 'jf transfer-files' command.")
	return
}

func (tcc *TransferConfigCommand) runPreChecks(sourceServicesManager, targetServicesManager artifactory.ArtifactoryServicesManager) error {
	// Warn if default admin:password credentials are exist in the source server
	_, err := commandsUtils.IsDefaultCredentials(targetServicesManager, tcc.sourceServerDetails.ArtifactoryUrl)
	if err != nil {
		return err
	}
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	if _, err = validateMinVersionAndDifferentServers(sourceServicesManager, tcc.sourceServerDetails, tcc.targetServerDetails); err != nil {
		return err
	}
	// Make sure that the target Artifactory is empty and the config-import plugin is installed
	if err := tcc.validateTargetServer(targetServicesManager); err != nil {
		return err
	}

	// Download and decrypt the config XML from the source Artifactory
	configXml, err := tcc.getConfigXml(sourceServicesManager)
	if err != nil {
		return err
	}

	// Remove filtered repositories
	configXml, err = configxmlutils.RemoveNonIncludedRepositories(configXml, tcc.getRepoFilter())
	if err != nil {
		return err
	}

	return tcc.NewPreChecksRunner(&targetServicesManager, configXml).Run(context.Background(), tcc.targetServerDetails)
}

func (tcc *TransferConfigCommand) printWarnings(sourceServicesManager artifactory.ArtifactoryServicesManager) (continueTransfer bool, err error) {
	// Prompt message
	promptMsg := "This command will transfer Artifactory config data:\n" +
		fmt.Sprintf("From %s - <%s>\n", coreutils.PrintBold("Source"), tcc.sourceServerDetails.ArtifactoryUrl) +
		fmt.Sprintf("To %s - <%s>\n", coreutils.PrintBold("Target"), tcc.targetServerDetails.ArtifactoryUrl) +
		"This action will wipe out all Artifactory content in the target.\n" +
		"Make sure that you're using strong credentials in your source platform (for example - having the default admin:password credentials isn't recommended).\n" +
		"Those credentials will be transferred to your SaaS target platform.\n" +
		"Are you sure you want to continue?"

	if !coreutils.AskYesNo(promptMsg, false) {
		return false, nil
	}

	// Check if there is a configured user using default credentials in the source platform.
	isDefaultCredentials, err := commandsUtils.IsDefaultCredentials(sourceServicesManager, tcc.sourceServerDetails.ArtifactoryUrl)
	if err != nil {
		return false, err
	}
	if isDefaultCredentials && !coreutils.AskYesNo("Are you sure you want to continue?", false) {
		return false, nil
	}
	return true, nil
}

// Make sure the target Artifactory is empty, by counting the number of the users. If it is bigger than 1, return an error.
// Also, make sure that the config-import plugin is installed
func (tcc *TransferConfigCommand) validateTargetServer(targetServicesManager artifactory.ArtifactoryServicesManager) error {
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

// Make sure source and target Artifactory URLs are different.
// Also make sure that the source Artifactory version is sufficient.
// Returns the source Artifactory version.
func validateMinVersionAndDifferentServers(sourceServicesManager artifactory.ArtifactoryServicesManager, sourceServerDetails, targetServerDetails *config.ServerDetails) (string, error) {
	log.Info("Verifying minimum version of the source server...")
	sourceArtifactoryVersion, err := sourceServicesManager.GetVersion()
	if err != nil {
		return "", err
	}
	err = coreutils.ValidateMinimumVersion(coreutils.Artifactory, sourceArtifactoryVersion, minArtifactoryVersion)
	if err != nil {
		return "", err
	}
	// Avoid exporting and importing to the same server
	log.Info("Verifying source and target servers are different...")
	if sourceServerDetails.GetArtifactoryUrl() == targetServerDetails.GetArtifactoryUrl() {
		return "", errorutils.CheckErrorf("The source and target Artifactory servers are identical, but should be different.")
	}

	return sourceArtifactoryVersion, nil
}

func (tcc *TransferConfigCommand) verifyConfigImportPlugin(targetServicesManager artifactory.ArtifactoryServicesManager) error {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.targetServerDetails.GetArtifactoryUrl())

	// Create rtDetails
	rtDetails, err := commandsUtils.CreateArtifactoryClientDetails(targetServicesManager)
	if err != nil {
		return err
	}

	// Get config-import plugin version
	configImportVersionUrl := artifactoryUrl + commandsUtils.PluginsExecuteRestApi + "configImportVersion"
	configImportPluginVersion, err := commandsUtils.GetTransferPluginVersion(targetServicesManager.Client(), configImportVersionUrl, "config-import", commandsUtils.Target, rtDetails)
	if err != nil {
		return err
	}
	log.Info("config-import plugin version: " + configImportPluginVersion)

	// Execute 'GET /api/plugins/execute/checkPermissions'
	resp, body, _, err := targetServicesManager.Client().SendGet(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"checkPermissions"+tcc.getWorkingDirParam(), false, rtDetails)
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
func (tcc *TransferConfigCommand) NewPreChecksRunner(targetServiceManager *artifactory.ArtifactoryServicesManager, configXml string) (runner *commandsUtils.PreCheckRunner) {
	runner = commandsUtils.NewPreChecksRunner()

	// Add pre checks here
	runner.AddCheck(commandsUtils.NewRemoteRepositoryCheck(targetServiceManager, configXml))

	return
}

// Download and decrypt artifactory.config.xml from the source Artifactory server.
// It is safer to not store the decrypted artifactory.config.xml file in the file system, and therefore we only keep it in memory.
func (tcc *TransferConfigCommand) getConfigXml(sourceServiceManager artifactory.ArtifactoryServicesManager) (configXml string, err error) {
	var wasEncrypted bool
	if wasEncrypted, err = sourceServiceManager.DeactivateKeyEncryption(); err != nil {
		return "", err
	}
	defer func() {
		if !wasEncrypted {
			return
		}
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
// 2. Remove federated members of federated repositories
func (tcc *TransferConfigCommand) modifyConfigXml(configXml string) (string, bool, error) {
	var err error
	configXml, err = configxmlutils.RemoveNonIncludedRepositories(configXml, tcc.getRepoFilter())
	if err != nil {
		return "", false, err
	}
	return configxmlutils.RemoveFederatedMembers(configXml)
}

// Import from the input buffer to the target Artifactory
func (tcc *TransferConfigCommand) importToTargetArtifactory(targetServicesManager artifactory.ArtifactoryServicesManager, buffer *bytes.Buffer) (err error) {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(tcc.targetServerDetails.GetArtifactoryUrl())
	var timestamp []byte

	// Create rtDetails
	rtDetails, err := commandsUtils.CreateArtifactoryClientDetails(targetServicesManager)
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
			resp, body, err := targetServicesManager.Client().SendPost(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"configImport"+tcc.getWorkingDirParam(), buffer.Bytes(), rtDetails)
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
	if strings.Contains(string(body), "[ERROR]") {
		return errorutils.CheckErrorf("Errors detected during config import. Hint: You can skip transferring some Artifactory repositories by using the '--exclude-repos' command option. Run 'jf rt transfer-config -h' for more information.")
	}
	return nil
}

func (tcc *TransferConfigCommand) createImportPollingAction(targetServicesManager artifactory.ArtifactoryServicesManager, rtDetails *httputils.HttpClientDetails, artifactoryUrl string, importTimestamp []byte) httputils.PollingAction {
	return func() (shouldStop bool, responseBody []byte, err error) {
		// Get config import status
		resp, body, err := targetServicesManager.Client().SendPost(artifactoryUrl+commandsUtils.PluginsExecuteRestApi+"configImportStatus"+tcc.getWorkingDirParam(), importTimestamp, rtDetails)
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
		newServerDetails := tcc.targetServerDetails
		newServerDetails.SetUser(tcc.sourceServerDetails.GetUser())
		newServerDetails.SetPassword(tcc.sourceServerDetails.GetPassword())
		newServerDetails.SetAccessToken(tcc.sourceServerDetails.GetAccessToken())

		targetServicesManager, err = utils.CreateServiceManager(newServerDetails, -1, 0, false)
		if err != nil {
			return true, nil, err
		}
		rtDetails, err = commandsUtils.CreateArtifactoryClientDetails(targetServicesManager)
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

func (tcc *TransferConfigCommand) getWorkingDirParam() string {
	if tcc.workingDir != "" {
		return "?params=workingDir=" + tcc.workingDir
	}
	return ""
}
