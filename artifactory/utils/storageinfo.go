package utils

import (
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
)

const (
	serviceManagerRetriesPerRequest         = 3
	serviceManagerRetriesWaitPerRequest int = int(1 * time.Second)
)

var getRepoSummaryPollingTimeout = 10 * time.Minute
var getRepoSummaryPollingInterval = 10 * time.Second

type StorageInfoManager struct {
	serviceManager artifactory.ArtifactoryServicesManager
}

func NewStorageInfoManager(serverDetails *config.ServerDetails) *StorageInfoManager {
	serviceManager, err := CreateServiceManager(serverDetails, serviceManagerRetriesPerRequest, serviceManagerRetriesWaitPerRequest, false)
	if err != nil {
		return nil
	}
	return &StorageInfoManager{serviceManager: serviceManager}
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
// This method must be running after CalculateStorageInfo.
// repoKey - The repository key
func (sim *StorageInfoManager) GetRepoSummary(repoKey string) (*serviceUtils.RepositorySummary, error) {
	var retVal *serviceUtils.RepositorySummary
	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         getRepoSummaryPollingTimeout,
		PollingInterval: getRepoSummaryPollingInterval,
		MsgPrefix:       "Waiting for storage info calculation completion",
		PollingAction: func() (shouldStop bool, responseBody []byte, err error) {
			storageInfo, err := sim.GetStorageInfo()
			if err != nil {
				return true, []byte{}, err
			}

			for _, repoSummary := range storageInfo.RepositoriesSummaryList {
				if repoSummary.RepoKey == repoKey {
					retVal = &repoSummary
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
