package state

import (
	"fmt"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	milliSecsInSecond               = 1000
	bytesInMB                       = 1024 * 1024
	bytesPerMilliSecToMBPerSec      = float64(milliSecsInSecond) / float64(bytesInMB)
	minTransferTimeToShowEstimation = time.Minute * 5
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
	// Total transferred bytes since the beginning of the current transfer running
	CurrentTotalTransferredBytes uint64 `json:"current_total_transferred_bytes,omitempty"`
	// The state manager
	stateManager *TransferStateManager
}

func (tem *TimeEstimationManager) AddChunkStatus(chunkStatus api.ChunkStatus, durationMillis int64) {
	if durationMillis == 0 {
		return
	}

	tem.addDataChunkStatus(chunkStatus, durationMillis)
}

func (tem *TimeEstimationManager) addDataChunkStatus(chunkStatus api.ChunkStatus, durationMillis int64) {
	var chunkSizeBytes int64
	for _, file := range chunkStatus.Files {
		if file.Status != api.Fail {
			tem.CurrentTotalTransferredBytes += uint64(file.SizeBytes)
		}
		if (file.Status == api.Success || file.Status == api.SkippedLargeProps) && !file.ChecksumDeployed {
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
	if len(tem.LastSpeeds) == 0 {
		return "Not available yet"
	}
	return fmt.Sprintf("%.3f MB/s", tem.getSpeed())
}

// GetEstimatedRemainingTimeString gets the estimated remaining time in an easy-to-read string.
// Return "Not available yet" in the following cases:
// 1. 5 minutes not passed since the beginning of the transfer
// 2. No files transferred
// 3. The transfer speed is less than 1 byte per second
func (tem *TimeEstimationManager) GetEstimatedRemainingTimeString() string {
	remainingTimeSec := tem.getEstimatedRemainingSeconds()
	if remainingTimeSec == 0 {
		return "Not available yet"
	}

	return SecondsToLiteralTime(int64(remainingTimeSec), "About ")
}

func (tem *TimeEstimationManager) getEstimatedRemainingSeconds() uint64 {
	if tem.CurrentTotalTransferredBytes == 0 {
		// No files transferred
		return 0
	}
	duration := time.Since(tem.stateManager.startTimestamp)
	if duration < minTransferTimeToShowEstimation {
		// 5 minutes not yet passed
		return 0
	}

	transferredBytesInSeconds := tem.CurrentTotalTransferredBytes / uint64(duration.Seconds())
	if transferredBytesInSeconds == 0 {
		// Less than 1 byte per second
		return 0
	}
	remainingBytes := tem.stateManager.OverallTransfer.TotalSizeBytes - tem.stateManager.OverallTransfer.TransferredSizeBytes
	return uint64(remainingBytes) / transferredBytesInSeconds
}
