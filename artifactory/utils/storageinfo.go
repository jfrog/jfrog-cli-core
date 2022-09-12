package utils

import (
	"context"
	"strconv"
	"strings"
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

func NewStorageInfoManager(ctx context.Context, serverDetails *config.ServerDetails) (*StorageInfoManager, error) {
	serviceManager, err := CreateServiceManagerWithContext(ctx, serverDetails, false, 0, serviceManagerRetriesPerRequest, serviceManagerRetriesWaitPerRequestMilliSecs)
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

// GetReposTotalSize gets the total size of all passed repositories, in bytes.
// This method must be called after CalculateStorageInfo.
// The result of this function might not be accurate!
func (sim *StorageInfoManager) GetReposTotalSize(repoKeys ...string) (int64, error) {
	var totalSize int64
	reposCounted := 0
	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         getRepoSummaryPollingTimeout,
		PollingInterval: getRepoSummaryPollingInterval,
		MsgPrefix:       "Waiting for storage info calculation completion",
		PollingAction: func() (shouldStop bool, responseBody []byte, err error) {
			storageInfo, err := sim.GetStorageInfo()
			if err != nil {
				return true, nil, err
			}
			reposMap := make(map[string]bool)
			for _, repoKey := range repoKeys {
				reposMap[repoKey] = true
			}
			for _, repoSummary := range storageInfo.RepositoriesSummaryList {
				if reposMap[repoSummary.RepoKey] {
					reposCounted++
					if repoSummary.UsedSpaceInBytes.String() != "" {
						sizeToAdd, err := repoSummary.UsedSpaceInBytes.Int64()
						if err != nil {
							totalSize += sizeToAdd
							continue
						}
					}

					sizeToAdd, err := convertStorageSizeStringToBytes(repoSummary.UsedSpace)
					if err != nil {
						return true, nil, err
					}
					totalSize += sizeToAdd
				}
			}
			return reposCounted == len(repoKeys), nil, nil
		},
	}
	_, err := pollingExecutor.Execute()
	if reposCounted < len(repoKeys) && err == nil {
		return totalSize, errorutils.CheckErrorf("one or more of the requested repositories were not found")
	}
	return totalSize, err
}

func convertStorageSizeStringToBytes(sizeStr string) (int64, error) {
	usedSpaceParts := strings.Split(sizeStr, " ")
	if len(usedSpaceParts) != 2 {
		return 0, errorutils.CheckErrorf("could not parse size string '%s'", sizeStr)
	}
	sizeInUnit, err := strconv.ParseFloat(usedSpaceParts[0], 64)
	if err != nil {
		return 0, err
	}
	var sizeInBytes float64

	switch usedSpaceParts[1] {
	case "bytes":
		sizeInBytes = sizeInUnit
	case "KB":
		sizeInBytes = sizeInUnit * 1024
	case "MB":
		sizeInBytes = sizeInUnit * 1024 * 1024
	case "GB":
		sizeInBytes = sizeInUnit * 1024 * 1024 * 1024
	case "TB":
		sizeInBytes = sizeInUnit * 1024 * 1024 * 1024 * 1024
	default:
		return 0, errorutils.CheckErrorf("could not parse size string '%s'", sizeStr)
	}
	return int64(sizeInBytes), nil
}
