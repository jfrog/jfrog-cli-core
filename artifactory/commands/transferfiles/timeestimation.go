package transferfiles

import (
	"fmt"
)

const (
	secondsInMinute            = 60
	secondsInHour              = 60 * secondsInMinute
	secondsInDay               = 24 * secondsInHour
	milliSecsInSecond          = 1000
	bytesInMB                  = 1024 * 1024
	bytesPerMilliSecToMBPerSec = float64(milliSecsInSecond) / float64(bytesInMB)
)

var numOfSpeedsToKeepPerWorkingThread = 10

type timeEstimationManager struct {
	// Speeds of the last done chunks, in bytes/ms
	lastSpeeds []float64
	// Sum of the speeds in lastSpeeds. The speeds are in bytes/ms.
	lastSpeedsSum float64
	// The last calculated sum of speeds, in bytes/ms
	speedsAverage float64
	// Total transferred data, in bytes
	transferredSizeBytes int64
	// Total size to transfer, in bytes
	totalSizeBytes int64
	// True if remaining time estimation is unavailable
	timeEstimationUnavailable bool
	// Map of repository keys and the amount of data (in bytes) transferred in each of them since the last update of the state.
	transferredSizeSinceStateUpdate map[string]int64
}

func newTimeEstimationManager(totalSizeBytes, transferredSizeBytes int64) *timeEstimationManager {
	return &timeEstimationManager{totalSizeBytes: totalSizeBytes, transferredSizeBytes: transferredSizeBytes, transferredSizeSinceStateUpdate: make(map[string]int64)}
}

func (tem *timeEstimationManager) addChunkStatus(chunkStatus ChunkStatus, workingThreads int, includedInTotalSize bool) {
	if chunkStatus.DurationMillis == 0 {
		return
	}
	var sizeSum int64
	for _, file := range chunkStatus.Files {
		if file.Status == Success {
			if includedInTotalSize {
				tem.transferredSizeBytes += file.SizeBytes
				tem.transferredSizeSinceStateUpdate[file.Repo] += file.SizeBytes
			}
			if !file.ChecksumDeployed {
				sizeSum += file.SizeBytes
			}
		}
	}

	// If no files were uploaded regularly (with no errors and not checksum-deployed), don't use this chunk for the time estimation calculation.
	if sizeSum == 0 {
		return
	}

	speed := calculateChunkSpeed(workingThreads, sizeSum, chunkStatus.DurationMillis)
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
	if len(tem.lastSpeeds) == 0 {
		return "Not available yet"
	}
	return fmt.Sprintf("%.3f MB/s", tem.getSpeed())
}

// getEstimatedRemainingTime gets the estimated remaining time in seconds.
func (tem *timeEstimationManager) getEstimatedRemainingTime() int64 {
	if tem.speedsAverage == 0 {
		return 0
	}
	remainingTime := (tem.totalSizeBytes - tem.transferredSizeBytes) / int64(tem.speedsAverage)
	// Convert from milliseconds to seconds
	return remainingTime / milliSecsInSecond
}

// getEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
func (tem *timeEstimationManager) getEstimatedRemainingTimeString() string {
	if tem.timeEstimationUnavailable {
		return "Not available in this phase"
	}
	if len(tem.lastSpeeds) == 0 {
		return "Not available yet"
	}
	remainingTimeSec := tem.getEstimatedRemainingTime()
	remainingDays := remainingTimeSec / secondsInDay
	remainingDaysInSecs := remainingDays * secondsInDay
	remainingHours := (remainingTimeSec - remainingDaysInSecs) / secondsInHour
	if remainingDays >= 1 {
		return fmt.Sprintf("About %d days and %d hours", remainingDays, remainingHours)
	}
	remainingHoursInSecs := remainingHours * secondsInHour
	remainingMinutes := (remainingTimeSec - remainingHoursInSecs) / secondsInMinute
	if remainingHours >= 1 {
		return fmt.Sprintf("About %d hours and %d minutes", remainingHours, remainingMinutes)
	}
	if remainingMinutes >= 1 {
		return fmt.Sprintf("About %d minutes", remainingMinutes)
	}
	return "Less than a minute"
}

func (tem *timeEstimationManager) setTimeEstimationUnavailable(timeEstimationUnavailable bool) {
	tem.timeEstimationUnavailable = timeEstimationUnavailable
}

//lint:ignore U1000 will be used in a different pull request
func (tem *timeEstimationManager) saveTransferredSizeInState() error {
	for repoKey, sizeToAdd := range tem.transferredSizeSinceStateUpdate {
		err := incRepoTransferredSizeBytes(repoKey, sizeToAdd)
		if err != nil {
			return err
		}
	}
	tem.transferredSizeSinceStateUpdate = make(map[string]int64)
	return nil
}

func isTimeEstimationEnabled() bool {
	return false
}
