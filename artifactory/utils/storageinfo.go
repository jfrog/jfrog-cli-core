package utils

import (
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
)

const (
	serviceManagerRetriesPerRequest                  = 3
	serviceManagerRetriesWaitPerRequestMilliSecs int = 1000
)

var getRepoSummaryPollingTimeout = 10 * time.Minute
var getRepoSummaryPollingInterval = 10 * time.Second

type StorageInfoManager struct {
	serviceManager artifactory.ArtifactoryServicesManager
}

func NewStorageInfoManager(serverDetails *config.ServerDetails) (*StorageInfoManager, error) {
	serviceManager, err := CreateServiceManager(serverDetails, serviceManagerRetriesPerRequest, serviceManagerRetriesWaitPerRequestMilliSecs, false)
	if err != nil {
		return nil, err
	}
	return &StorageInfoManager{serviceManager: serviceManager}, nil
}

// Start calculating storage info in Artifactory
func (sim *StorageInfoManager) CalculateStorageInfo() error {
	return sim.serviceManager.CalculateStorageInfo()
}

// Get storage info from Artifactory
func (sim *StorageInfoManager) GetStorageInfo() (*utils.StorageInfo, error) {
	return sim.serviceManager.GetStorageInfo()
}

// Get repository summary from the storage info.
// This method must be called after CalculateStorageInfo.
// repoKey - The repository key
func (sim *StorageInfoManager) GetRepoSummary(repoKey string) (*utils.RepositorySummary, error) {
	var retVal *utils.RepositorySummary
	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         getRepoSummaryPollingTimeout,
		PollingInterval: getRepoSummaryPollingInterval,
		MsgPrefix:       "Waiting for storage info calculation completion",
		PollingAction: func() (shouldStop bool, responseBody []byte, err error) {
			storageInfo, err := sim.GetStorageInfo()
			if err != nil {
				return true, []byte{}, err
			}

			for i, repoSummary := range storageInfo.RepositoriesSummaryList {
				if repoSummary.RepoKey == repoKey {
					retVal = &storageInfo.RepositoriesSummaryList[i]
					return true, []byte{}, nil
				}
			}
			return false, []byte{}, nil
		},
	}
	_, err := pollingExecutor.Execute()
	if retVal == nil && err == nil {
		return nil, errorutils.CheckErrorf("could not find repository '%s' in the repositories summary", repoKey)
	}
	return retVal, err
}
