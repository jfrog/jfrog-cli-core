package transfer

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strconv"
)

type TransferSettingsCommand struct {
}

func NewTransferSettingsCommand() *TransferSettingsCommand {
	return &TransferSettingsCommand{}
}

func (tst *TransferSettingsCommand) Run() error {
	var threadsNumberInput string
	ioutils.ScanFromConsole("Choose the threads number", &threadsNumberInput, "")
	threadsNumber, err := strconv.Atoi(threadsNumberInput)
	if err != nil || threadsNumber < 1 {
		return errorutils.CheckError(errors.New("the threads number must be a numeric positive value"))
	}
	conf := &utils.TransferSettings{ThreadsNumber: threadsNumber}
	err = utils.SaveTransferSettings(conf)
	if err != nil {
		return err
	}
	log.Output("The settings were saved successfully. It might take a while until they take effect.")
	return nil
}

func (tst *TransferSettingsCommand) ServerDetails() (*config.ServerDetails, error) {
	// Since it's a local command, usage won't be reported.
	return nil, nil
}

func (tst *TransferSettingsCommand) CommandName() string {
	return "rt_transfer_set_threads"
}
