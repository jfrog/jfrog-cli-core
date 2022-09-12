package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
)

const secondsInMinute = 60
const secondsInHour = 60 * secondsInMinute
const secondsInDay = 24 * secondsInHour

type timeEstimationManager struct {
	// Max number of last calculated speeds to make an average from
	maxSpeedsSliceLength int
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
}

func newTimeEstimationManager(totalSizeBytes, transferredSizeBytes int64) *timeEstimationManager {
	return &timeEstimationManager{totalSizeBytes: totalSizeBytes, transferredSizeBytes: transferredSizeBytes, maxSpeedsSliceLength: utils.DefaultThreads}
}

func (tem *timeEstimationManager) addChunkStatus(chunkStatus ChunkStatus, workingThreads int) {
	if chunkStatus.DurationMillis == 0 {
		return
	}
	var sizeSum int64
	for _, file := range chunkStatus.Files {
		if file.Status == Success {
			tem.transferredSizeBytes += file.SizeBytes
			if !file.ChecksumDeployed {
				sizeSum += file.SizeBytes
			}
		}
	}

	// If no files were uploaded regularly, don't use this chunk in the time estimation
	if sizeSum == 0 {
		return
	}

	speed := float64(workingThreads) * float64(sizeSum) / float64(chunkStatus.DurationMillis)
	tem.lastSpeeds = append(tem.lastSpeeds, speed)
	tem.lastSpeedsSum += speed
	for len(tem.lastSpeeds) > tem.maxSpeedsSliceLength {
		// Remove the oldest calculated speed
		tem.lastSpeedsSum -= tem.lastSpeeds[0]
		tem.lastSpeeds = tem.lastSpeeds[1:]
	}
	// Calculate speed in bytes/ms
	tem.speedsAverage = tem.lastSpeedsSum / float64(len(tem.lastSpeeds))
}

// getSpeed gets the transfer speed, in MB/s.
func (tem *timeEstimationManager) getSpeed() float64 {
	// Convert from bytes/ms to MB/s
	return tem.speedsAverage * 1000 / 1024 / 1024
}

// getEstimatedRemainingTime gets the estimated remaining time in seconds.
func (tem *timeEstimationManager) getEstimatedRemainingTime() int64 {
	if tem.speedsAverage == 0 {
		return 0
	}
	remainingTime := (tem.totalSizeBytes - tem.transferredSizeBytes) / int64(tem.speedsAverage)
	// Convert from milliseconds to seconds
	return remainingTime / 1000
}

// getEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
func (tem *timeEstimationManager) getEstimatedRemainingTimeString() string {
	remainingTimeSec := tem.getEstimatedRemainingTime()
	remainingDays := remainingTimeSec / secondsInDay
	remainingDaysInSecs := remainingDays * secondsInDay
	remainingHours := (remainingTimeSec - remainingDaysInSecs) / secondsInHour
	if remainingDays > 0 {
		return fmt.Sprintf("About %d days and %d hours", remainingDays, remainingHours)
	}
	remainingHoursInSecs := remainingHours * secondsInHour
	remainingMinutes := (remainingTimeSec - remainingDaysInSecs - remainingHoursInSecs) / secondsInMinute
	return fmt.Sprintf("About %d hours and %d minutes", remainingHours, remainingMinutes)
}

func (tem *timeEstimationManager) setMaxSpeedsSliceLength(maxSpeedsSliceLength int) {
	tem.maxSpeedsSliceLength = maxSpeedsSliceLength
}
