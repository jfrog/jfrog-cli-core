package commands

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Command interface {
	// Runs the command
	Run() error
	// Returns the Artifactory details. The usage report is sent to this Artifactory server.
	RtDetails() (*config.ArtifactoryDetails, error)
	// The command name for the usage report.
	CommandName() string
}

func Exec(command Command) error {
	channel := make(chan bool)
	// Triggers the report usage.
	go reportUsage(command, channel)
	// Invoke the command interface
	err := command.Run()
	// Waits for the signal from the report usage to be done.
	<-channel
	return err
}

func reportUsage(command Command, channel chan<- bool) {
	defer signalReportUsageFinished(channel)
	reportUsage, err := clientutils.GetBoolEnvValue(coreutils.ReportUsage, true)
	if err != nil {
		log.Debug(usage.ReportUsagePrefix + err.Error())
		return
	}
	if reportUsage {
		rtDetails, err := command.RtDetails()
		if err != nil {
			log.Debug(usage.ReportUsagePrefix + err.Error())
			return
		}
		if rtDetails != nil {
			log.Debug(usage.ReportUsagePrefix + "Sending info...")
			serviceManager, err := utils.CreateServiceManager(rtDetails, false)
			if err != nil {
				log.Debug(usage.ReportUsagePrefix + err.Error())
				return
			}
			err = usage.SendReportUsage(coreutils.GetUserAgent(), command.CommandName(), serviceManager)
			if err != nil {
				log.Debug(err.Error())
				return
			}
		}
	} else {
		log.Debug("Usage info is disabled.")
	}
}

// Set to true when the report usage func exits
func signalReportUsageFinished(ch chan<- bool) {
	ch <- true
}
