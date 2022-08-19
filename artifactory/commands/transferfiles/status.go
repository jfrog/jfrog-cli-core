package transferfiles

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const sizeUnits = "KMGTPE"

func ShowStatus() error {
	var output strings.Builder
	defer func() {
		log.Output(output.String())
	}()

	startTimestamp, err := state.GetStartTimestamp()
	if err != nil {
		return err
	}
	if startTimestamp == 0 {
		addString(&output, "🔴", "Status", "Not running", 0)
		return nil
	}
	stateManager, err := state.NewTransferStateManager(true)
	if err != nil {
		return err
	}
	if err := addOverallStatus(stateManager, &output, startTimestamp); err != nil {
		return err
	}
	if stateManager.CurrentRepo != "" {
		output.WriteString("\n")
		setRepositoryStatus(stateManager, &output)
	}
	return nil
}

func addOverallStatus(stateManager *state.TransferStateManager, output *strings.Builder, startTimestamp int64) error {
	addTitle(output, "Overall Transfer Status")
	addString(output, "🟢", "Status", "Transferring files", 3)
	addString(output, "⏱️ ", "Start time", time.Unix(0, startTimestamp).Format(time.Stamp), 3)
	addString(output, "🗄 ", "Transferred size", sizeToString(stateManager.TransferredSizeBytes)+" / "+sizeToString(stateManager.TotalSizeBytes)+calcPercentageInt64(stateManager.TransferredSizeBytes, stateManager.TotalSizeBytes), 2)
	addString(output, "📦", "Transferred repositories", fmt.Sprintf("%d / %d", stateManager.TransferredUnits, stateManager.TotalUnits)+calcPercentage(stateManager.TransferredUnits, stateManager.TotalUnits), 1)

	transferSettings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	if transferSettings == nil {
		transferSettings = &utils.TransferSettings{}
	}
	addString(output, "🧵", "Max working threads", strconv.Itoa(transferSettings.CalcNumberOfThreads(false)), 1)
	return nil
}

func calcPercentage(transferred, total int) string {
	return calcPercentageInt64(int64(transferred), int64(total))
}

func calcPercentageInt64(transferred, total int64) string {
	return fmt.Sprintf(" (%.1f%%)", float64(transferred)/float64(total)*100)
}

func setRepositoryStatus(stateManager *state.TransferStateManager, output *strings.Builder) {
	addTitle(output, "Current Repository Status")
	addString(output, "🏷 ", "Name", stateManager.CurrentRepo, 3)
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
			addString(output, "🔢", "Phase", "Transferring all files in the repository (1/3)", 3)
		} else {
			addString(output, "🔢", "Phase", "Retrying transfer failures (3/3)", 3)
		}
		addString(output, "🗄 ", "Transferred data", sizeToString(currentRepo.TransferredSizeBytes)+" / "+sizeToString(currentRepo.TotalSizeBytes)+calcPercentageInt64(currentRepo.TransferredSizeBytes, currentRepo.TotalSizeBytes), 2)
		addString(output, "📄", "Transferred files", fmt.Sprintf("%d / %d", currentRepo.TransferredUnits, currentRepo.TotalUnits)+calcPercentage(currentRepo.TransferredUnits, currentRepo.TotalUnits), 2)
	case FilesDiffPhase:
		addString(output, "🔢", "Phase", "Transferring newly created and modified files (2/3)", 3)
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
