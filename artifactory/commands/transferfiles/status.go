package transferfiles

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const sizeUnits = "KMGTPE"

func ShowStatus() error {
	var output strings.Builder
	stateManager, err := state.NewTransferStateManager(true)
	if err != nil {
		return err
	}

	isRunning, err := stateManager.InitStartTimestamp()
	if err != nil {
		return err
	}
	if !isRunning {
		addString(&output, "🔴", "Status", "Not running", 0)
		log.Output(output.String())
		return nil
	}
	isStopping, err := isStopping()
	if err != nil {
		return err
	}
	if isStopping {
		addString(&output, "🟡", "Status", "Stopping", 0)
		log.Output(output.String())
		return nil
	}

	if err = addOverallStatus(stateManager, &output, stateManager.GetRunningTimeString()); err != nil {
		return err
	}
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

func addOverallStatus(stateManager *state.TransferStateManager, output *strings.Builder, runningTime string) error {
	addTitle(output, "Overall Transfer Status")
	addString(output, coreutils.RemoveEmojisIfNonSupportedTerminal("🟢"), "Status", "Running", 3)
	addString(output, "🏃", "Running for", runningTime, 3)
	addString(output, "🗄 ", "Storage", sizeToString(stateManager.OverallTransfer.TransferredSizeBytes)+" / "+sizeToString(stateManager.OverallTransfer.TotalSizeBytes)+calcPercentageInt64(stateManager.OverallTransfer.TransferredSizeBytes, stateManager.OverallTransfer.TotalSizeBytes), 3)
	addString(output, "📦", "Repositories", fmt.Sprintf("%d / %d", stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits)+calcPercentageInt64(stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits), 2)
	addString(output, "🧵", "Working threads", strconv.Itoa(stateManager.WorkingThreads), 2)
	addString(output, "⚡", "Transfer speed", stateManager.GetSpeedString(), 2)
	estimatedRemainingTime, err := stateManager.GetEstimatedRemainingTimeString()
	if err != nil {
		return err
	}
	addString(output, "⌛", "Estimated time remaining", estimatedRemainingTime, 1)
	failureTxt := strconv.FormatUint(stateManager.TransferFailures, 10)
	if stateManager.TransferFailures > 0 {
		failureTxt += " (" + progressbar.RetryFailureContentNote + ")"
	}
	addString(output, "❌", "Transfer failures", failureTxt, 2)
	if stateManager.SkippedSourceGone > 0 {
		addString(output, "⏭ ", "Skipped (deleted at source)", strconv.FormatUint(stateManager.SkippedSourceGone, 10)+" (items that no longer exist in the source; listed in the errors summary CSV, not retried)", 1)
	}
	return nil
}

func calcPercentageInt64(transferred, total int64) string {
	if transferred == 0 || total == 0 {
		return ""
	}
	return fmt.Sprintf(" (%.1f%%)", float64(transferred)/float64(total)*100)
}

func setRepositoryStatus(stateManager *state.TransferStateManager, output *strings.Builder) {
	addTitle(output, "Current Repository Status")
	addString(output, "🏷 ", "Name", stateManager.CurrentRepoKey, 3)
	currentRepo := stateManager.CurrentRepo
	switch stateManager.CurrentRepoPhase {
	case api.Phase1, api.Phase3:
		if stateManager.CurrentRepoPhase == api.Phase1 {
			addString(output, "🔢", "Phase", "Transferring files in the repository (1/3)", 3)
		} else {
			addString(output, "🔢", "Phase", "Retrying transfer failures and transfer delayed files (3/3)", 3)
		}
		addString(output, "🗄 ", "Storage", sizeToString(currentRepo.Phase1Info.TransferredSizeBytes)+" / "+sizeToString(currentRepo.Phase1Info.TotalSizeBytes)+calcPercentageInt64(currentRepo.Phase1Info.TransferredSizeBytes, currentRepo.Phase1Info.TotalSizeBytes), 3)
		addString(output, "📄", "Files", fmt.Sprintf("%d / %d", currentRepo.Phase1Info.TransferredUnits, currentRepo.Phase1Info.TotalUnits)+calcPercentageInt64(currentRepo.Phase1Info.TransferredUnits, currentRepo.Phase1Info.TotalUnits), 3)
	case api.Phase2:
		addString(output, "🔢", "Phase", "Transferring newly created and modified files (2/3)", 3)
	}
	if stateManager.CurrentRepoPhase == api.Phase1 {
		addString(output, "📁", "Visited folders", strconv.FormatUint(stateManager.VisitedFolders, 10), 2)
	}
	delayedTxt := strconv.FormatUint(stateManager.DelayedFiles, 10)
	if stateManager.DelayedFiles > 0 {
		delayedTxt += " (" + progressbar.DelayedFilesContentNote + ")"
	}
	addString(output, "✋", "Delayed files", delayedTxt, 2)
}

func addTitle(output *strings.Builder, title string) {
	output.WriteString(coreutils.PrintBoldTitle(title + "\n"))
}

func addString(output *strings.Builder, emoji, key, value string, tabsCount int) {
	indentation := strings.Repeat("\t", tabsCount)
	if indentation == "" {
		indentation = " "
	}
	if len(emoji) > 0 {
		if coreutils.IsWindows() {
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
