package transferfiles

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const sizeUnits = "KMGTPE"

func ShowStatus() error {
	var output strings.Builder
	runningTime, isRunning, err := state.GetRunningTime()
	if err != nil {
		return err
	}
	if !isRunning {
		addString(&output, "🔴", "Status", "Not running", 0, coreutils.IsWindows())
		log.Output(output.String())
		return nil
	}
	isStopping, err := isStopping()
	if err != nil {
		return err
	}
	if isStopping {
		addString(&output, "🟡", "Status", "Stopping", 0, coreutils.IsWindows())
		log.Output(output.String())
		return nil
	}

	stateManager, err := state.NewTransferStateManager(true)
	if err != nil {
		return err
	}
	addOverallStatus(stateManager, &output, runningTime)
	if stateManager.CurrentRepoKey != "" {
		transferState, exists, err := state.LoadTransferState(stateManager.CurrentRepoKey, false)
		if err != nil {
			return err
		}
		if !exists {
			return errorutils.CheckErrorf("could not find the state file of repository '%s'. Aborting", stateManager.CurrentRepoKey)
		}
		stateManager.TransferState = transferState
		output.WriteString("\n")
		setRepositoryStatus(stateManager, &output)
	}
	log.Output(output.String())
	return nil
}

func isStopping() (bool, error) {
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return false, err
	}

	return fileutils.IsFileExists(filepath.Join(transferDir, StopFileName), false)
}

func addOverallStatus(stateManager *state.TransferStateManager, output *strings.Builder, runningTime string) {
	windows := coreutils.IsWindows()
	addTitle(output, "Overall Transfer Status")
	addString(output, coreutils.RemoveEmojisIfNonSupportedTerminal("🟢"), "Status", "Running", 3, windows)
	addString(output, "🏃", "Running for", runningTime, 3, windows)
	addString(output, "🗄 ", "Storage", sizeToString(stateManager.OverallTransfer.TransferredSizeBytes)+" / "+sizeToString(stateManager.OverallTransfer.TotalSizeBytes)+calcPercentageInt64(stateManager.OverallTransfer.TransferredSizeBytes, stateManager.OverallTransfer.TotalSizeBytes), 3, windows)
	addString(output, "📦", "Repositories", fmt.Sprintf("%d / %d", stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits)+calcPercentageInt64(stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits), 2, windows)
	addString(output, "🧵", "Working threads", strconv.Itoa(stateManager.WorkingThreads), 2, windows)
	addString(output, "⚡", "Transfer speed", stateManager.GetSpeedString(), 2, windows)
	addString(output, "⌛", "Estimated time remaining", stateManager.GetEstimatedRemainingTimeString(), 1, windows)
	failureTxt := strconv.FormatUint(uint64(stateManager.TransferFailures), 10)
	if stateManager.TransferFailures > 0 {
		failureTxt += " (" + "In Phase 3 and in subsequent executions, we'll retry transferring the failed files." + ")"
	}
	addString(output, "❌", "Transfer failures", failureTxt, 2, windows)
}

func calcPercentageInt64(transferred, total int64) string {
	if transferred == 0 || total == 0 {
		return ""
	}
	return fmt.Sprintf(" (%.1f%%)", float64(transferred)/float64(total)*100)
}

func setRepositoryStatus(stateManager *state.TransferStateManager, output *strings.Builder) {
	windows := coreutils.IsWindows()
	addTitle(output, "Current Repository Status")
	addString(output, "🏷 ", "Name", stateManager.CurrentRepoKey, 2, windows)
	currentRepo := stateManager.CurrentRepo
	switch stateManager.CurrentRepoPhase {
	case api.Phase1, api.Phase3:
		if stateManager.CurrentRepoPhase == api.Phase1 {
			addString(output, "🔢", "Phase", "Transferring all files in the repository (1/3)", 2, windows)
		} else {
			addString(output, "🔢", "Phase", "Retrying transfer failures (3/3)", 2, windows)
		}
		addString(output, "🗄 ", "Storage", sizeToString(currentRepo.Phase1Info.TransferredSizeBytes)+" / "+sizeToString(currentRepo.Phase1Info.TotalSizeBytes)+calcPercentageInt64(currentRepo.Phase1Info.TransferredSizeBytes, currentRepo.Phase1Info.TotalSizeBytes), 2, windows)
		addString(output, "📄", "Files", fmt.Sprintf("%d / %d", currentRepo.Phase1Info.TransferredUnits, currentRepo.Phase1Info.TotalUnits)+calcPercentageInt64(currentRepo.Phase1Info.TransferredUnits, currentRepo.Phase1Info.TotalUnits), 2, windows)
	case api.Phase2:
		addString(output, "🔢", "Phase", "Transferring newly created and modified files (2/3)", 2, windows)
	}
}

func addTitle(output *strings.Builder, title string) {
	output.WriteString(coreutils.PrintBoldTitle(title + "\n"))
}

func addString(output *strings.Builder, emoji, key, value string, tabsCount int, windows bool) {
	indentation := strings.Repeat("\t", tabsCount)
	if indentation == "" {
		indentation = " "
	}
	if len(emoji) > 0 {
		if windows {
			emoji = "●"
		}
		emoji += " "
	}
	key = emoji + key + ":"
	// PrintBold removes emojis if they are unsupported. After they are removed, the string is also trimmed, so we should avoid adding trailing spaces to the key.
	output.WriteString(coreutils.PrintBold(key))
	output.WriteString(indentation + value + "\n")
}

func sizeToString(sizeInBytes int64) string {
	var divider int64 = 1024
	sizeUnitIndex := 0
	for ; sizeUnitIndex < len(sizeUnits)-1 && (sizeInBytes >= divider<<10); sizeUnitIndex++ {
		divider <<= 10
	}
	return fmt.Sprintf("%.1f %ciB", float64(sizeInBytes)/float64(divider), sizeUnits[sizeUnitIndex])
}
