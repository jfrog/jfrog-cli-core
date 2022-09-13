package transferfiles

import (
	"fmt"
)

const secondsInMinute = 60
const secondsInHour = 60 * secondsInMinute
const secondsInDay = 24 * secondsInHour

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
}

func newTimeEstimationManager(totalSizeBytes, transferredSizeBytes int64) *timeEstimationManager {
	return &timeEstimationManager{totalSizeBytes: totalSizeBytes, transferredSizeBytes: transferredSizeBytes}
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

	speed := float64(workingThreads) * float64(sizeSum) / float64(chunkStatus.DurationMillis)
	tem.lastSpeeds = append(tem.lastSpeeds, speed)
	tem.lastSpeedsSum += speed
	lastSpeedsSliceLen := workingThreads * 10
	for len(tem.lastSpeeds) > lastSpeedsSliceLen {
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

// getSpeed gets the transfer speed in an easy-to-read string.
func (tem *timeEstimationManager) getSpeedString() string {
	if len(tem.lastSpeeds) == 0 {
		return "Data is not yet available, since only metadata has been transferred till now"
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
	return remainingTime / 1000
}

// getEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
func (tem *timeEstimationManager) getEstimatedRemainingTimeString() string {
	if tem.timeEstimationUnavailable {
		return "Not available in this phase"
	}
	if len(tem.lastSpeeds) == 0 {
		return "Data is not yet available, since only metadata has been transferred till now"
	}
	remainingTimeSec := tem.getEstimatedRemainingTime()
	remainingDays := remainingTimeSec / secondsInDay
	remainingDaysInSecs := remainingDays * secondsInDay
	remainingHours := (remainingTimeSec - remainingDaysInSecs) / secondsInHour
	if remainingDays >= 1 {
		return fmt.Sprintf("About %d days and %d hours", remainingDays, remainingHours)
	}
	remainingHoursInSecs := remainingHours * secondsInHour
	remainingMinutes := (remainingTimeSec - remainingDaysInSecs - remainingHoursInSecs) / secondsInMinute
	if remainingHours >= 1 {
		return fmt.Sprintf("About %d hours and %d minutes", remainingHours, remainingMinutes)
	}
	return fmt.Sprintf("About %d minutes", remainingMinutes)
}

func (tem *timeEstimationManager) setTimeEstimationUnavailable(timeEstimationUnavailable bool) {
	tem.timeEstimationUnavailable = timeEstimationUnavailable
}
