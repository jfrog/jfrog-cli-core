package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	secondsInMinute            = 60
	secondsInHour              = 60 * secondsInMinute
	secondsInDay               = 24 * secondsInHour
	milliSecsInSecond          = 1000
	bytesInMB                  = 1024 * 1024
	bytesPerMilliSecToMBPerSec = float64(milliSecsInSecond) / float64(bytesInMB)
	// Precalculated average index time per build info, in seconds.
	buildInfoAverageIndexTimeSec = 1.25
)

type timeTypeSingular string

const (
	day    timeTypeSingular = "day"
	hour   timeTypeSingular = "hour"
	minute timeTypeSingular = "minute"
)

var numOfSpeedsToKeepPerWorkingThread = 10

type timeEstimationManager struct {
	// Speeds of the last done chunks, in bytes/ms
	lastSpeeds []float64
	// Sum of the speeds in lastSpeeds. The speeds are in bytes/ms.
	lastSpeedsSum float64
	// The last calculated sum of speeds, in bytes/ms
	speedsAverage float64
	// True if remaining time estimation is unavailable
	timeEstimationUnavailable bool
	// True if handling a build info repository.
	buildInfoRepo bool
	// Data estimated remaining time is saved so that it can be used when handling a build-info repository and speed cannot be calculated.
	dataEstimatedRemainingTime int64
	// The state manager
	stateManager *state.TransferStateManager
	// Build info transferred and totals for the estimation.
	transferredBuildInfoFiles int64
	totalBuildInfoFiles       int64
}

func newTimeEstimationManager(stateManager *state.TransferStateManager, totalBiFiles int64) *timeEstimationManager {
	return &timeEstimationManager{stateManager: stateManager, totalBuildInfoFiles: totalBiFiles}
}

func (tem *timeEstimationManager) addChunkStatus(chunkStatus ChunkStatus, durationMillis int64) {
	if durationMillis == 0 {
		return
	}

	if tem.buildInfoRepo {
		tem.addBuildInfoChunkStatus(chunkStatus)
		return
	}
	tem.addDataChunkStatus(chunkStatus, durationMillis)
}

func (tem *timeEstimationManager) addDataChunkStatus(chunkStatus ChunkStatus, durationMillis int64) {
	var chunkSizeBytes int64
	for _, file := range chunkStatus.Files {
		if file.Status == Success && !file.ChecksumDeployed {
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
	tem.lastSpeeds = append(tem.lastSpeeds, speed)
	tem.lastSpeedsSum += speed
	lastSpeedsSliceLen := workingThreads * numOfSpeedsToKeepPerWorkingThread
	for len(tem.lastSpeeds) > lastSpeedsSliceLen {
		// Remove the oldest calculated speed
		tem.lastSpeedsSum -= tem.lastSpeeds[0]
		tem.lastSpeeds = tem.lastSpeeds[1:]
	}
	// Calculate speed in bytes/ms
	tem.speedsAverage = tem.lastSpeedsSum / float64(len(tem.lastSpeeds))
}

func (tem *timeEstimationManager) addBuildInfoChunkStatus(chunkStatus ChunkStatus) {
	for _, file := range chunkStatus.Files {
		if file.Status == Success {
			tem.transferredBuildInfoFiles++
		}
	}
}

func calculateChunkSpeed(workingThreads int, chunkSizeSum, chunkDuration int64) float64 {
	return float64(workingThreads) * float64(chunkSizeSum) / float64(chunkDuration)
}

// getSpeed gets the transfer speed, in MB/s.
func (tem *timeEstimationManager) getSpeed() float64 {
	// Convert from bytes/ms to MB/s
	return tem.speedsAverage * bytesPerMilliSecToMBPerSec
}

// getSpeed gets the transfer speed in an easy-to-read string.
func (tem *timeEstimationManager) getSpeedString() string {
	if tem.buildInfoRepo {
		return "Not available when handling a build-info repository"
	}
	if len(tem.lastSpeeds) == 0 {
		return "Not available yet"
	}
	return fmt.Sprintf("%.3f MB/s", tem.getSpeed())
}

// getEstimatedRemainingTime gets the estimated remaining time in seconds.
// The estimated remaining time is the sum of:
// 1. Data estimated remaining time, derived by the average speed and remaining data size.
// 2. Build info estimated remaining time, derived by a precalculated average time per build info.
func (tem *timeEstimationManager) getEstimatedRemainingTime() (int64, error) {
	err := tem.calculateDataEstimatedRemainingTime()
	if err != nil {
		return 0, err
	}
	return tem.dataEstimatedRemainingTime + tem.getBuildInfoEstimatedRemainingTime(), nil
}

// calculateDataEstimatedRemainingTime calculates the data estimated remaining time in seconds, and sets it to the corresponding
// variable in the estimation manager.
func (tem *timeEstimationManager) calculateDataEstimatedRemainingTime() error {
	// If a build info repository is currently being handled, use the data estimated time previously calculated.
	// Else, start calculating when the speeds average is set.
	if tem.buildInfoRepo || tem.speedsAverage == 0 {
		return nil
	}
	transferredSizeBytes, err := tem.stateManager.GetTransferredSizeBytes()
	if err != nil {
		return err
	}

	if tem.stateManager.TotalSizeBytes <= transferredSizeBytes {
		tem.dataEstimatedRemainingTime = 0
		return nil
	}

	// We only convert to int64 at the end to avoid a scenario where the conversion of speedsAverage returns zero.
	remainingTime := float64(tem.stateManager.TotalSizeBytes-transferredSizeBytes) / tem.speedsAverage
	// Convert from milliseconds to seconds.
	tem.dataEstimatedRemainingTime = int64(remainingTime) / milliSecsInSecond
	return nil
}

func (tem *timeEstimationManager) getBuildInfoEstimatedRemainingTime() int64 {
	if tem.totalBuildInfoFiles <= tem.transferredBuildInfoFiles {
		return 0
	}

	workingThreads, err := tem.getWorkingThreadsForBuildInfoEstimation()
	if err != nil {
		log.Error("Couldn't calculate time estimation:", err.Error())
		return 0
	}

	remainingBiFiles := float64(tem.totalBuildInfoFiles - tem.transferredBuildInfoFiles)
	remainingTime := remainingBiFiles * buildInfoAverageIndexTimeSec / float64(workingThreads)
	return int64(remainingTime)
}

func (tem *timeEstimationManager) getWorkingThreadsForBuildInfoEstimation() (int, error) {
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

// getEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
func (tem *timeEstimationManager) getEstimatedRemainingTimeString() string {
	if tem.timeEstimationUnavailable {
		return "Not available in this phase"
	}
	if !tem.buildInfoRepo && len(tem.lastSpeeds) == 0 {
		return "Not available yet"
	}
	remainingTimeSec, err := tem.getEstimatedRemainingTime()
	if err != nil {
		return err.Error()
	}
	remainingDays := remainingTimeSec / secondsInDay
	remainingDaysInSecs := remainingDays * secondsInDay
	remainingHours := (remainingTimeSec - remainingDaysInSecs) / secondsInHour
	if remainingDays >= 1 {
		return getEstimationWithRemainder(remainingDays, remainingHours, day, hour)
	}

	remainingHoursInSecs := remainingHours * secondsInHour
	remainingMinutes := (remainingTimeSec - remainingHoursInSecs) / secondsInMinute
	if remainingHours >= 1 {
		return getEstimationWithRemainder(remainingHours, remainingMinutes, hour, minute)
	}

	if remainingMinutes >= 1 {
		return getEstimationWithRemainder(remainingMinutes, 0, minute, "")
	}
	return "Less than a minute"
}

// Get the time estimation as string, with the remainder added only if it is non-zero.
// For example "About 2 hours and 1 minute"
func getEstimationWithRemainder(mainAmount, remainderAmount int64, mainType, remainderType timeTypeSingular) string {
	estimation := "About " + getTimeSingularOrPlural(mainAmount, mainType)
	if remainderAmount > 0 {
		estimation += " and " + getTimeSingularOrPlural(remainderAmount, remainderType)
	}
	return estimation
}

// Returns the time amount followed by its type, with 's' for plural if needed.
// For example '1 hour' or '2 hours'.
func getTimeSingularOrPlural(timeAmount int64, timeType timeTypeSingular) string {
	result := fmt.Sprintf("%d %s", timeAmount, timeType)
	if timeAmount > 1 {
		result += "s"
	}
	return result
}

func (tem *timeEstimationManager) setTimeEstimationUnavailable(timeEstimationUnavailable bool) {
	tem.timeEstimationUnavailable = timeEstimationUnavailable
}

func (tem *timeEstimationManager) setBuildInfoRepo(buildInfoRepo bool) {
	tem.buildInfoRepo = buildInfoRepo
}
