package utils

import (
	"context"
	"errors"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"strconv"
	"strings"
	"time"
)

const (
	serviceManagerRetriesPerRequest                  = 3
	serviceManagerRetriesWaitPerRequestMilliSecs int = 1000
	storageInfoRepoMissingError                      = "one or more of the requested repositories were not found"

	bytesInKB = 1024
	bytesInMB = 1024 * bytesInKB
	bytesInGB = 1024 * bytesInMB
	bytesInTB = 1024 * bytesInGB
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

func (sim *StorageInfoManager) GetServiceManager() artifactory.ArtifactoryServicesManager {
	return sim.serviceManager
}

// Start calculating storage info in Artifactory
func (sim *StorageInfoManager) CalculateStorageInfo() error {
	return sim.serviceManager.CalculateStorageInfo()
}

// Get storage info from Artifactory
func (sim *StorageInfoManager) GetStorageInfo() (*utils.StorageInfo, error) {
	return sim.serviceManager.GetStorageInfo()
}

// Get Service Id from Artifactory
func (sim *StorageInfoManager) GetServiceId() (string, error) {
	return sim.serviceManager.GetServiceId()
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
	if retVal == nil && (err == nil || errors.As(err, &clientUtils.RetryExecutorTimeoutError{})) {
		return nil, errorutils.CheckErrorf("could not find repository '%s' in the repositories summary", repoKey)
	}
	return retVal, err
}

// GetReposTotalSizeAndFiles gets the total size (bytes) and files of all passed repositories.
// This method must be called after CalculateStorageInfo.
// The result of this function might not be accurate!
func (sim *StorageInfoManager) GetReposTotalSizeAndFiles(repoKeys ...string) (totalSize, totalFiles int64, err error) {
	reposCounted := 0
	reposSet := datastructures.MakeSet[string]()
	for _, repoKey := range repoKeys {
		reposSet.Add(repoKey)
	}
	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         getRepoSummaryPollingTimeout,
		PollingInterval: getRepoSummaryPollingInterval,
		MsgPrefix:       "Waiting for storage info calculation completion",
		PollingAction: func() (shouldStop bool, responseBody []byte, err error) {
			// Reset counters between polling attempts.
			totalSize = 0
			reposCounted = 0
			totalFiles = 0

			storageInfo, err := sim.GetStorageInfo()
			if err != nil {
				return true, nil, err
			}
			for i, repoSummary := range storageInfo.RepositoriesSummaryList {
				if reposSet.Exists(repoSummary.RepoKey) {
					reposCounted++
					sizeToAdd, err := GetUsedSpaceInBytes(&storageInfo.RepositoriesSummaryList[i])
					if err != nil {
						return true, nil, err
					}
					totalSize += sizeToAdd

					filesToAdd, err := GetFilesCountFromRepositorySummary(&storageInfo.RepositoriesSummaryList[i])
					if err != nil {
						return true, nil, err
					}
					totalFiles += filesToAdd
				}
			}
			return reposCounted == len(repoKeys), nil, nil
		},
	}
	_, err = pollingExecutor.Execute()
	if reposCounted < len(repoKeys) && (err == nil || errors.As(err, &clientUtils.RetryExecutorTimeoutError{})) {
		return totalSize, totalFiles, errorutils.CheckErrorf(storageInfoRepoMissingError)
	}
	return totalSize, totalFiles, err
}

func GetFilesCountFromRepositorySummary(repoSummary *utils.RepositorySummary) (int64, error) {
	files, err := repoSummary.FilesCount.Int64()
	return files, errorutils.CheckError(err)
}

func GetUsedSpaceInBytes(repoSummary *utils.RepositorySummary) (int64, error) {
	if repoSummary.UsedSpaceInBytes.String() != "" {
		size, err := repoSummary.UsedSpaceInBytes.Int64()
		return size, errorutils.CheckError(err)
	}

	return convertStorageSizeStringToBytes(repoSummary.UsedSpace)
}

func convertStorageSizeStringToBytes(sizeStr string) (int64, error) {
	usedSpaceParts := strings.Fields(sizeStr)
	if len(usedSpaceParts) != 2 {
		return 0, errorutils.CheckErrorf("could not parse size string '%s'", sizeStr)
	}
	// The ReplaceAll removes ',' from the number, for example: 1,004.64 -> 1004.64
	sizeInUnit, err := strconv.ParseFloat(strings.ReplaceAll(usedSpaceParts[0], ",", ""), 64)
	if err != nil {
		return 0, errorutils.CheckError(err)
	}
	var sizeInBytes float64

	switch usedSpaceParts[1] {
	case "bytes":
		sizeInBytes = sizeInUnit
	case "KB":
		sizeInBytes = sizeInUnit * bytesInKB
	case "MB":
		sizeInBytes = sizeInUnit * bytesInMB
	case "GB":
		sizeInBytes = sizeInUnit * bytesInGB
	case "TB":
		sizeInBytes = sizeInUnit * bytesInTB
	default:
		return 0, errorutils.CheckErrorf("could not parse size string '%s'", sizeStr)
	}
	return int64(sizeInBytes), nil
}
