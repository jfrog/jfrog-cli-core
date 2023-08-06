package state

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	milliSecsInSecond          = 1000
	bytesInMB                  = 1024 * 1024
	bytesPerMilliSecToMBPerSec = float64(milliSecsInSecond) / float64(bytesInMB)
	// Precalculated average index time per build info, in seconds. This constant is used to estimate the processing time of all
	// the build info files about to be transferred. Since the build info indexing time is not related to its file size,
	// the estimation approach we use with data is irrelevant.
	buildInfoAverageIndexTimeSec = 1.25
)

type timeTypeSingular string

const (
	day    timeTypeSingular = "day"
	hour   timeTypeSingular = "hour"
	minute timeTypeSingular = "minute"
)

var numOfSpeedsToKeepPerWorkingThread = 10

type TimeEstimationManager struct {
	// Speeds of the last done chunks, in bytes/ms
	LastSpeeds []float64 `json:"last_speeds,omitempty"`
	// Sum of the speeds in LastSpeeds. The speeds are in bytes/ms.
	LastSpeedsSum float64 `json:"last_speeds_sum,omitempty"`
	// The last calculated sum of speeds, in bytes/ms
	SpeedsAverage float64 `json:"speeds_average,omitempty"`
	// Data estimated remaining time is saved so that it can be used when handling a build-info repository and speed cannot be calculated.
	DataEstimatedRemainingTime int64 `json:"data_estimated_remaining_time,omitempty"`
	// The state manager
	stateManager *TransferStateManager
}

func (tem *TimeEstimationManager) AddChunkStatus(chunkStatus api.ChunkStatus, durationMillis int64) {
	// Build info repository requires no action here (transferred counter is updated in the state manager and no other calculation is needed).
	if durationMillis == 0 || tem.stateManager.BuildInfoRepo {
		return
	}

	tem.addDataChunkStatus(chunkStatus, durationMillis)
}

func (tem *TimeEstimationManager) addDataChunkStatus(chunkStatus api.ChunkStatus, durationMillis int64) {
	var chunkSizeBytes int64
	for _, file := range chunkStatus.Files {
		if file.Status == api.Success && !file.ChecksumDeployed {
			chunkSizeBytes += file.SizeBytes
		}
	}

	// If no files were uploaded regularly (with no errors and not checksum-deployed), don't use this chunk for the time estimation calculation.
	if chunkSizeBytes == 0 {
		return
	}

	workingThreads, err := tem.stateManager.GetWorkingThreads()
	if err != nil {
		log.Error("Couldn't calculate time estimation:", err.Error())
		return
	}
	speed := calculateChunkSpeed(workingThreads, chunkSizeBytes, durationMillis)
	tem.LastSpeeds = append(tem.LastSpeeds, speed)
	tem.LastSpeedsSum += speed
	lastSpeedsSliceLen := workingThreads * numOfSpeedsToKeepPerWorkingThread
	for len(tem.LastSpeeds) > lastSpeedsSliceLen {
		// Remove the oldest calculated speed
		tem.LastSpeedsSum -= tem.LastSpeeds[0]
		tem.LastSpeeds = tem.LastSpeeds[1:]
	}
	if len(tem.LastSpeeds) == 0 {
		tem.SpeedsAverage = 0
		return
	}
	// Calculate speed in bytes/ms
	tem.SpeedsAverage = tem.LastSpeedsSum / float64(len(tem.LastSpeeds))
}

func calculateChunkSpeed(workingThreads int, chunkSizeSum, chunkDuration int64) float64 {
	return float64(workingThreads) * float64(chunkSizeSum) / float64(chunkDuration)
}

// getSpeed gets the transfer speed, in MB/s.
func (tem *TimeEstimationManager) getSpeed() float64 {
	// Convert from bytes/ms to MB/s
	return tem.SpeedsAverage * bytesPerMilliSecToMBPerSec
}

// GetSpeedString gets the transfer speed in an easy-to-read string.
func (tem *TimeEstimationManager) GetSpeedString() string {
	if tem.stateManager.BuildInfoRepo {
		return "Not available while transferring a build-info repository"
	}
	if len(tem.LastSpeeds) == 0 {
		return "Not available yet"
	}
	return fmt.Sprintf("%.3f MB/s", tem.getSpeed())
}

// getEstimatedRemainingTime gets the estimated remaining time in seconds.
// The estimated remaining time is the sum of:
// 1. Data estimated remaining time, derived by the average speed and remaining data size.
// 2. Build info estimated remaining time, derived by a precalculated average time per build info.
func (tem *TimeEstimationManager) getEstimatedRemainingTime() (int64, error) {
	err := tem.calculateDataEstimatedRemainingTime()
	if err != nil {
		return 0, err
	}
	return tem.DataEstimatedRemainingTime + tem.getBuildInfoEstimatedRemainingTime(), nil
}

// calculateDataEstimatedRemainingTime calculates the data estimated remaining time in seconds, and sets it to the corresponding
// variable in the estimation manager.
func (tem *TimeEstimationManager) calculateDataEstimatedRemainingTime() error {
	// If a build info repository is currently being handled, use the data estimated time previously calculated.
	// Else, start calculating when the speeds average is set.
	if tem.stateManager.BuildInfoRepo || tem.SpeedsAverage == 0 {
		return nil
	}
	transferredSizeBytes, err := tem.stateManager.GetTransferredSizeBytes()
	if err != nil {
		return err
	}

	// In case we reach a situation where we transfer more data than expected, we cannot estimate how long transferring the remaining data will take.
	if tem.stateManager.OverallTransfer.TotalSizeBytes <= transferredSizeBytes {
		tem.DataEstimatedRemainingTime = 0
		return nil
	}

	// We only convert to int64 at the end to avoid a scenario where the conversion of SpeedsAverage returns zero.
	remainingTime := float64(tem.stateManager.OverallTransfer.TotalSizeBytes-transferredSizeBytes) / tem.SpeedsAverage
	// Convert from milliseconds to seconds.
	tem.DataEstimatedRemainingTime = int64(remainingTime) / milliSecsInSecond
	return nil
}

func (tem *TimeEstimationManager) getBuildInfoEstimatedRemainingTime() int64 {
	if tem.stateManager.OverallBiFiles.TotalUnits <= tem.stateManager.OverallBiFiles.TransferredUnits {
		return 0
	}

	workingThreads, err := tem.getWorkingThreadsForBuildInfoEstimation()
	if err != nil {
		log.Error("Couldn't calculate time estimation:", err.Error())
		return 0
	}

	remainingBiFiles := float64(tem.stateManager.OverallBiFiles.TotalUnits - tem.stateManager.OverallBiFiles.TransferredUnits)
	remainingTime := remainingBiFiles * buildInfoAverageIndexTimeSec / float64(workingThreads)
	return int64(remainingTime)
}

func (tem *TimeEstimationManager) getWorkingThreadsForBuildInfoEstimation() (int, error) {
	workingThreads, err := tem.stateManager.GetWorkingThreads()
	if err != nil {
		return 0, err
	}
	// If the uploader didn't start working, temporarily display estimation according to one thread.
	if workingThreads == 0 {
		return 1, nil
	}
	// If currently handling a data repository and the number of threads is high, show build info estimation according to the build info threads limit.
	if workingThreads > utils.MaxBuildInfoThreads {
		return utils.MaxBuildInfoThreads, nil
	}
	return workingThreads, nil
}

// GetEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
func (tem *TimeEstimationManager) GetEstimatedRemainingTimeString() string {
	if !tem.isTimeEstimationAvailable() {
		return "Not available in this phase"
	}
	if !tem.stateManager.BuildInfoRepo && len(tem.LastSpeeds) == 0 {
		return "Not available yet"
	}
	remainingTimeSec, err := tem.getEstimatedRemainingTime()
	if err != nil {
		return err.Error()
	}

	return SecondsToLiteralTime(remainingTimeSec, "About ")
}

func (tem *TimeEstimationManager) isTimeEstimationAvailable() bool {
	return tem.stateManager.CurrentRepoPhase == api.Phase1 || tem.stateManager.CurrentRepoPhase == api.Phase3
}
