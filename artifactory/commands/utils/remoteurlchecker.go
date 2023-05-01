package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type RemoteUrlCheckStatus string

const (
	longPropertyCheckName           = "Remote repositories URL connectivity"
	remoteUrlCheckPollingTimeout    = 30 * time.Minute
	remoteUrlCheckPollingInterval   = 5 * time.Second
	remoteUrlCheckRetries           = 3
	remoteUrlCheckIntervalMilliSecs = 10000
)

type remoteRepoSettings struct {
	Key         string `json:"key,omitempty"`
	Url         string `json:"url,omitempty"`
	RepoType    string `json:"repo_type,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	QueryParams string `json:"query_params,omitempty"`
}

type remoteUrlResponse struct {
	Status                   RemoteUrlCheckStatus     `json:"status,omitempty"`
	InaccessibleRepositories []inaccessibleRepository `json:"inaccessible_repositories,omitempty"`
	CheckedRepositories      uint                     `json:"checked_repositories,omitempty"`
	TotalRepositories        uint                     `json:"total_repositories,omitempty"`
}

type inaccessibleRepository struct {
	RepoKey    string `json:"repo_key,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Url        string `json:"url,omitempty"`
}

// Run remote repository URLs accessibility test before transferring configuration from one Artifactory to another
type RemoteRepositoryCheck struct {
	targetServicesManager *artifactory.ArtifactoryServicesManager
	remoteRepositories    []interface{}
}

func NewRemoteRepositoryCheck(targetServicesManager *artifactory.ArtifactoryServicesManager, remoteRepositories []interface{}) *RemoteRepositoryCheck {
	return &RemoteRepositoryCheck{targetServicesManager, remoteRepositories}
}

func (rrc *RemoteRepositoryCheck) Name() string {
	return longPropertyCheckName
}

func (rrc *RemoteRepositoryCheck) ExecuteCheck(args RunArguments) (passed bool, err error) {
	remoteUrlRequest, err := rrc.createRemoteUrlRequest()
	if err != nil {
		return false, err
	}
	inaccessibleRepositories, err := rrc.doCheckRemoteRepositories(args, remoteUrlRequest)
	if err != nil {
		return false, err
	}
	if len(*inaccessibleRepositories) == 0 {
		return true, nil
	}
	return false, handleFailureRun(*inaccessibleRepositories)
}

// Create the remote URL request from the received remote repository details from Artifactory
func (rrc *RemoteRepositoryCheck) createRemoteUrlRequest() ([]remoteRepoSettings, error) {
	remoteUrlRequests := make([]remoteRepoSettings, len(rrc.remoteRepositories))
	for i, remoteRepository := range rrc.remoteRepositories {
		// The remote repository interface is not necessarily of RemoteRepositoryBaseParams
		// type (can be a map) and therefore we marshal and unmarshal it.
		remoteRepositoryBytes, err := json.Marshal(remoteRepository)
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		var remoteRepositoryParams services.RemoteRepositoryBaseParams
		if err = json.Unmarshal(remoteRepositoryBytes, &remoteRepositoryParams); err != nil {
			return nil, errorutils.CheckError(err)
		}

		remoteUrlRequests[i] = remoteRepoSettings{
			Key:         remoteRepositoryParams.Key,
			Url:         remoteRepositoryParams.Url,
			RepoType:    remoteRepositoryParams.PackageType,
			Username:    remoteRepositoryParams.Username,
			Password:    remoteRepositoryParams.Password,
			QueryParams: remoteRepositoryParams.QueryParams,
		}
	}
	return remoteUrlRequests, nil
}

func (rrc *RemoteRepositoryCheck) doCheckRemoteRepositories(args RunArguments, remoteUrlRequest []remoteRepoSettings) (inaccessibleRepositories *[]inaccessibleRepository, err error) {
	artifactoryUrl := clientutils.AddTrailingSlashIfNeeded(args.ServerDetails.ArtifactoryUrl)

	body, err := json.Marshal(remoteUrlRequest)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	// Create rtDetails
	rtDetails, err := CreateArtifactoryClientDetails(*rrc.targetServicesManager)
	if err != nil {
		return nil, err
	}

	progressBar, err := rrc.startCheckRemoteRepositories(rtDetails, artifactoryUrl, args, body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if progressBar != nil {
			progressBar.GetBar().Abort(true)
		}
	}()

	// Wait for remote repositories check completion
	return rrc.waitForRemoteReposCheckCompletion(rtDetails, artifactoryUrl, progressBar)
}

func (rrc *RemoteRepositoryCheck) startCheckRemoteRepositories(rtDetails *httputils.HttpClientDetails, artifactoryUrl string, args RunArguments, requestBody []byte) (*progressbar.TasksProgressBar, error) {
	var response *remoteUrlResponse
	// Sometimes, POST api/plugins/execute/remoteRepositoriesCheck returns unexpectedly 404 errors, although the config-import plugin is installed.
	// To overcome this issue, we use a custom retryExecutor and not the default retry executor that retries only on HTTP errors >= 500.
	retryExecutor := clientutils.RetryExecutor{
		Context:                  args.Context,
		MaxRetries:               remoteUrlCheckRetries,
		RetriesIntervalMilliSecs: remoteUrlCheckIntervalMilliSecs,
		ErrorMessage:             fmt.Sprintf("Failed to start the remote repositories check in %s", artifactoryUrl),
		LogMsgPrefix:             "[Config import]",
		ExecutionHandler: func() (shouldRetry bool, err error) {
			// Start the remote repositories check process
			resp, responseBody, err := (*rrc.targetServicesManager).Client().SendPost(artifactoryUrl+PluginsExecuteRestApi+"remoteRepositoriesCheck", requestBody, rtDetails)
			if err != nil {
				return false, err
			}
			if err = errorutils.CheckResponseStatusWithBody(resp, responseBody, http.StatusOK); err != nil {
				return true, err
			}

			response, err = unmarshalRemoteUrlResponse(responseBody)
			return false, err
		},
	}

	if err := retryExecutor.Execute(); err != nil {
		return nil, err
	}

	if args.ProgressMng == nil {
		return nil, nil
	}
	return args.ProgressMng.NewTasksProgressBar(int64(response.TotalRepositories), coreutils.IsWindows(), "Remote repositories"), nil
}

func (rrc *RemoteRepositoryCheck) waitForRemoteReposCheckCompletion(rtDetails *httputils.HttpClientDetails, artifactoryUrl string, progressBar *progressbar.TasksProgressBar) (*[]inaccessibleRepository, error) {
	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         remoteUrlCheckPollingTimeout,
		PollingInterval: remoteUrlCheckPollingInterval,
		MsgPrefix:       "Waiting for remote repositories check completion in Artifactory server at " + artifactoryUrl,
		PollingAction:   rrc.createImportPollingAction(rtDetails, artifactoryUrl, progressBar),
	}

	body, err := pollingExecutor.Execute()
	if err != nil {
		return nil, err
	}
	response, err := unmarshalRemoteUrlResponse(body)
	if err != nil {
		return nil, err
	}
	return &response.InaccessibleRepositories, nil
}

func (rrc *RemoteRepositoryCheck) createImportPollingAction(rtDetails *httputils.HttpClientDetails, artifactoryUrl string, progressBar *progressbar.TasksProgressBar) httputils.PollingAction {
	return func() (shouldStop bool, responseBody []byte, err error) {
		// Get config import status
		resp, body, _, err := (*rrc.targetServicesManager).Client().SendGet(artifactoryUrl+PluginsExecuteRestApi+"remoteRepositoriesCheckStatus", true, rtDetails)
		if err != nil {
			return true, nil, err
		}

		// 200 - Import completed
		if resp.StatusCode == http.StatusOK {
			return true, body, nil
		}

		// 202 - Update status
		if resp.StatusCode == http.StatusAccepted {
			response, err := unmarshalRemoteUrlResponse(body)
			if err != nil {
				return true, nil, err
			}
			if progressBar != nil {
				delta := int64(response.CheckedRepositories) - progressBar.GetBar().Current()
				progressBar.GetBar().IncrInt64(delta)
			}
		}

		return false, nil, nil
	}
}

// Unmarshal response from Artifactory to remoteUrlResponse
func unmarshalRemoteUrlResponse(body []byte) (*remoteUrlResponse, error) {
	log.Debug(fmt.Sprintf("Response from Artifactory:\n%s", body))
	var response remoteUrlResponse
	err := json.Unmarshal(body, &response)
	return &response, errorutils.CheckError(err)
}

// Create csv summary of all the files with inaccessible remote repositories and log the result
func handleFailureRun(inaccessibleRepositories []inaccessibleRepository) (err error) {
	// Create summary
	csvPath, err := CreateCSVFile("inaccessible-repositories", inaccessibleRepositories, time.Now())
	if err != nil {
		log.Error("Couldn't create the inaccessible remote repository URLs CSV file", err)
		return
	}
	// Log result
	log.Info(fmt.Sprintf("Found %d inaccessible remote repository URLs. Check the summary CSV file in: %s", len(inaccessibleRepositories), csvPath))
	return
}
