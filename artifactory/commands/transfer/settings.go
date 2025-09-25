package transfer

import (
	"fmt"
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const MaxThreadsLimit = 1024

type TransferSettingsCommand struct {
}

func NewTransferSettingsCommand() *TransferSettingsCommand {
	return &TransferSettingsCommand{}
}

func (tst *TransferSettingsCommand) Run() error {
	currSettings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	var currThreadsNumber string
	if currSettings == nil {
		currThreadsNumber = strconv.Itoa(utils.DefaultThreads)
	} else {
		currThreadsNumber = strconv.Itoa(currSettings.ThreadsNumber)
	}
	var threadsNumberInput string
	ioutils.ScanFromConsole("Set the maximum number of working threads", &threadsNumberInput, currThreadsNumber)
	threadsNumber, err := strconv.Atoi(threadsNumberInput)
	if err != nil || threadsNumber < 1 || threadsNumber > MaxThreadsLimit {
		return errorutils.CheckErrorf("the value must be a number between 1 and %s", strconv.Itoa(MaxThreadsLimit))
	}
	conf := &utils.TransferSettings{ThreadsNumber: threadsNumber}
	err = utils.SaveTransferSettings(conf)
	if err != nil {
		return err
	}
	log.Output("The settings were saved successfully. It might take a few moments for the new settings to take effect.")
	log.Output(fmt.Sprintf("Note - For Build Info repositories, the number of worker threads will be limited to %d.", utils.MaxBuildInfoThreads))
	return nil
}

func (tst *TransferSettingsCommand) ServerDetails() (*config.ServerDetails, error) {
	// There's no need to report the usage of this command.
	return nil, nil
}

func (tst *TransferSettingsCommand) CommandName() string {
	return "rt_transfer_settings"
}
