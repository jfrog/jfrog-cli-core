package transferfiles

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const sizeUnits = "KMGTPE"

func ShowStatus() error {
	var output strings.Builder

	startTimestamp, err := state.GetStartTimestamp()
	if err != nil {
		return err
	}
	if startTimestamp == 0 {
		addString(&output, "🔴", "Status", "Not running", 0)
		log.Output(output.String())
		return nil
	}
	stateManager, err := state.NewTransferStateManager(true)
	if err != nil {
		return err
	}
	addOverallStatus(stateManager, &output, startTimestamp)
	if stateManager.CurrentRepo != "" {
		output.WriteString("\n")
		setRepositoryStatus(stateManager, &output)
	}
	log.Output(output.String())
	return nil
}

func addOverallStatus(stateManager *state.TransferStateManager, output *strings.Builder, startTimestamp int64) {
	addTitle(output, "Overall Transfer Status")
	addString(output, "🟢", "Status", "Running", 2)
	addString(output, "⏱️ ", "Start time", time.Unix(0, startTimestamp).Format(time.Stamp), 2)
	addString(output, "🗄 ", "Storage", sizeToString(stateManager.TransferredSizeBytes)+" / "+sizeToString(stateManager.TotalSizeBytes)+calcPercentageInt64(stateManager.TransferredSizeBytes, stateManager.TotalSizeBytes), 2)
	addString(output, "📦", "Repositories", fmt.Sprintf("%d / %d", stateManager.TransferredUnits, stateManager.TotalUnits)+calcPercentage(stateManager.TransferredUnits, stateManager.TotalUnits), 1)
	addString(output, "🧵", "Working threads", strconv.Itoa(stateManager.WorkingThreads), 1)
	addString(output, "❌ ", "Transfer failures", strconv.FormatUint(uint64(stateManager.TransferFailures), 10), 1)
}

func calcPercentage(transferred, total int) string {
	return calcPercentageInt64(int64(transferred), int64(total))
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
	case FullTransferPhase, ErrorsPhase:
		if stateManager.CurrentRepoPhase == FullTransferPhase {
			addString(output, "🔢", "Phase", "Transferring all files in the repository (1/3)", 2)
		} else {
			addString(output, "🔢", "Phase", "Retrying transfer failures (3/3)", 2)
		}
		addString(output, "🗄 ", "Storage", sizeToString(currentRepo.TransferredSizeBytes)+" / "+sizeToString(currentRepo.TotalSizeBytes)+calcPercentageInt64(currentRepo.TransferredSizeBytes, currentRepo.TotalSizeBytes), 2)
		addString(output, "📄", "Files", fmt.Sprintf("%d / %d", currentRepo.TransferredUnits, currentRepo.TotalUnits)+calcPercentage(currentRepo.TransferredUnits, currentRepo.TotalUnits), 2)
	case FilesDiffPhase:
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
