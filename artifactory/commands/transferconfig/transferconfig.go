package transferconfig

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferconfig/configxmlutils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
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
	sourceServerDetails *config.ServerDetails
	targetServerDetails *config.ServerDetails
	dryRun              bool
	force               bool
	// List of repositries to include in the import. If empty - include all.
	includedRepositories []string
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

func (tcc *TransferConfigCommand) SetIncludedRepositories(includedRepositories []string) *TransferConfigCommand {
	tcc.includedRepositories = includedRepositories
	return tcc
}

func (tcc *TransferConfigCommand) Run() (err error) {
	// Create config managers
	sourceServicesManager, err := utils.CreateServiceManager(tcc.sourceServerDetails, -1, 0, tcc.dryRun)
	if err != nil {
		return
	}
	targetServiceManager, err := utils.CreateServiceManager(tcc.targetServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	sourceArtifactoryVersion, err := sourceServicesManager.GetVersion()
	if err != nil {
		return
	}

	// Make sure that the source and targer Artifactory servers are different and that the target Artifactory is empty
	if err = tcc.validateArtifactoryServers(sourceServicesManager, sourceArtifactoryVersion); err != nil {
		return
	}

	// Run export on the source Artifactory
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

	// Download and decrypt the config XML from the source Artifactory
	configXml, err := tcc.getConfigXml(sourceServicesManager, sourceArtifactoryVersion)
	if err != nil {
		return
	}

	// Prepare the config XML to be imported to SaaS
	configXml, err = tcc.modifyConfigXml(configXml, tcc.sourceServerDetails.ArtifactoryUrl, tcc.targetServerDetails.AccessUrl)
	if err != nil {
		return
	}

	// Create an archive of the source Artifactory export in memory
	archiveConfig, err := archiveConfig(exportPath, configXml)
	if err != nil {
		return
	}

	// Import the archive to the target Artifactory
	return tcc.importToTargetArtifactory(targetServiceManager, archiveConfig)
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
		return errorutils.CheckErrorf("cowardly refusing to import the config to the target server, because it contains more than 2 users. You can bypass this rule by providing the --force flag.")
	}
	return nil
}

// Download and decrypt artifactory.config.xml from the source Artifactory server.
// It is safer to not store the decrypted artifactory.config.xml file in the file system and therefore we only keep it in memory.
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

	if err = os.Chmod(tempDir, 0755); err != nil {
		return "", func() error { return nil }, errorutils.CheckError(err)
	}

	// Do export
	trueValue := true
	exportParams := services.ExportParams{
		ExportPath:      tempDir,
		IncludeMetadata: &trueValue,
		Verbose:         &trueValue,
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
func (tcc *TransferConfigCommand) modifyConfigXml(configXml, sourceBaseUrl, targetBaseUrl string) (string, error) {
	var err error
	if len(tcc.includedRepositories) > 0 {
		configXml, err = configxmlutils.RemoveNonIncludedRepositories(configXml, tcc.includedRepositories)
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
	log.Info(fmt.Sprintf("\n====== Import log ======\n%s========================", body))
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

		// After 401 or 403, the server credentials are fixed and therefore we can run again
		return false, nil, nil
	}
}
