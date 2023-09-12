package transfer

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"os"
	"strconv"
	"strings"

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

	// Set the worker threads value.
	var currThreadsNumber string
	if currSettings == nil {
		currThreadsNumber = strconv.Itoa(utils.DefaultThreads)
	} else {
		currThreadsNumber = strconv.Itoa(currSettings.ThreadsNumber)
	}
	var threadsNumberInput string
	ioutils.ScanFromConsole("Set the maximum number of worker threads", &threadsNumberInput, currThreadsNumber)
	threadsNumber, err := strconv.Atoi(threadsNumberInput)
	if err != nil || threadsNumber < 1 || threadsNumber > MaxThreadsLimit {
		return errorutils.CheckErrorf("the worker threads value must be a number between 1 and " + strconv.Itoa(MaxThreadsLimit))
	}

	// Set the log level value.
	currLogLevel := tst.getCurrLogLevel(*currSettings)
	var logLevel string
	ioutils.ScanFromConsole("Set the log level (DEBUG, INFO, WARN or ERROR)", &logLevel, currLogLevel)
	logLevel = strings.ToUpper(logLevel)
	if err = tst.validateLogLevelValue(logLevel); err != nil {
		return err
	}

	conf := &utils.TransferSettings{
		ThreadsNumber: threadsNumber,
		LogLevel:      logLevel,
	}
	if err = utils.SaveTransferSettings(conf); err != nil {
		return err
	}
	log.Output("The settings were saved successfully. It might take a few moments for the new settings to take effect.")
	log.Output(fmt.Sprintf("Note - For Build Info repositories, the number of worker threads will be limited to %d.", utils.MaxBuildInfoThreads))
	return nil
}

func (tst *TransferSettingsCommand) getCurrLogLevel(settings utils.TransferSettings) string {
	currLogLevel := settings.LogLevel
	if currLogLevel == "" {
		currLogLevel = os.Getenv(coreutils.LogLevel)
	}
	if currLogLevel == "" {
		currLogLevel = "INFO"
	}
	return currLogLevel
}

func (tst *TransferSettingsCommand) validateLogLevelValue(loglevel string) error {
	if loglevel != "DEBUG" && loglevel != "INFO" && loglevel != "WARN" && loglevel != "ERROR" {
		return errorutils.CheckErrorf("the log level value is invalid")
	}
	return nil
}

func (tst *TransferSettingsCommand) ServerDetails() (*config.ServerDetails, error) {
	// There's no need to report the usage of this command.
	return nil, nil
}

func (tst *TransferSettingsCommand) CommandName() string {
	return "rt_transfer_settings"
}
