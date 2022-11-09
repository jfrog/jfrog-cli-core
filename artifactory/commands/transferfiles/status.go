package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strconv"
	"strings"
)

const sizeUnits = "KMGTPE"

func ShowStatus() error {
	var output strings.Builder

	runningTime, isRunning, err := state.GetRunningTime()
	if err != nil {
		return err
	}
	if !isRunning {
		addString(&output, "🔴", "Status", "Not running", 0)
		log.Output(output.String())
		return nil
	}
	stateManager, err := state.NewTransferStateManager(true)
	if err != nil {
		return err
	}
	addOverallStatus(stateManager, &output, runningTime)
	if stateManager.CurrentRepo != "" {
		output.WriteString("\n")
		setRepositoryStatus(stateManager, &output)
	}
	log.Output(output.String())
	return nil
}

func addOverallStatus(stateManager *state.TransferStateManager, output *strings.Builder, runningTime string) {
	addTitle(output, "Overall Transfer Status")
	addString(output, "🟢", "Status", "Running", 3)
	addString(output, "⏱️ ", "Running for", runningTime, 2)
	addString(output, "🗄 ", "Storage", sizeToString(stateManager.OverallTransfer.TransferredSizeBytes)+" / "+sizeToString(stateManager.OverallTransfer.TotalSizeBytes)+calcPercentageInt64(stateManager.OverallTransfer.TransferredSizeBytes, stateManager.OverallTransfer.TotalSizeBytes), 3)
	addString(output, "📦", "Repositories", fmt.Sprintf("%d / %d", stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits)+calcPercentageInt64(stateManager.TotalRepositories.TransferredUnits, stateManager.TotalRepositories.TotalUnits), 2)
	addString(output, "🧵", "Working threads", strconv.Itoa(stateManager.WorkingThreads), 2)
	addString(output, "⚡", "Transfer speed", stateManager.GetSpeedString(), 2)
	addString(output, "⌛", "Estimated time remaining", stateManager.GetEstimatedRemainingTimeString(), 1)
	failureTxt := strconv.FormatUint(uint64(stateManager.TransferFailures), 10)
	if stateManager.TransferFailures > 0 {
		failureTxt += " (" + RetryFailureContentNote + ")"
	}
	addString(output, "❌", "Transfer failures", failureTxt, 2)
}

func calcPercentageInt64(transferred, total int64) string {
	if transferred == 0 || total == 0 {
		return ""
	}
	return fmt.Sprintf(" (%.1f%%)", float64(transferred)/float64(total)*100)
}

func setRepositoryStatus(stateManager *state.TransferStateManager, output *strings.Builder) {
	addTitle(output, "Current Repository Status")
	addString(output, "🏷 ", "Name", stateManager.CurrentRepo, 2)
	var currentRepo state.Repository
	for _, repo := range stateManager.Repositories {
		if repo.Name == stateManager.CurrentRepo {
			currentRepo = repo
			break
		}
	}
	switch stateManager.CurrentRepoPhase {
	case api.FullTransferPhase, api.ErrorsPhase:
		if stateManager.CurrentRepoPhase == api.FullTransferPhase {
			addString(output, "🔢", "Phase", "Transferring all files in the repository (1/3)", 2)
		} else {
			addString(output, "🔢", "Phase", "Retrying transfer failures (3/3)", 2)
		}
		addString(output, "🗄 ", "Storage", sizeToString(currentRepo.TransferredSizeBytes)+" / "+sizeToString(currentRepo.TotalSizeBytes)+calcPercentageInt64(currentRepo.TransferredSizeBytes, currentRepo.TotalSizeBytes), 2)
		addString(output, "📄", "Files", fmt.Sprintf("%d / %d", currentRepo.TransferredUnits, currentRepo.TotalUnits)+calcPercentageInt64(currentRepo.TransferredUnits, currentRepo.TotalUnits), 2)
	case api.FilesDiffPhase:
		addString(output, "🔢", "Phase", "Transferring newly created and modified files (2/3)", 2)
	}
}

func addTitle(output *strings.Builder, title string) {
	output.WriteString(coreutils.PrintTitle(coreutils.PrintBold(title + "\n")))
}

func addString(output *strings.Builder, emoji, key, value string, tabsCount int) {
	indentation := strings.Repeat("\t", tabsCount)
	key += ": "
	if len(emoji) > 0 {
		key = emoji + " " + key
	}
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
