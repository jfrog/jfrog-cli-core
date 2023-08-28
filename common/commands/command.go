package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	usageReporter "github.com/jfrog/jfrog-cli-core/v2/utils/usage"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Command interface {
	// Runs the command
	Run() error
	// Returns the Server details. The usage report is sent to this server.
	ServerDetails() (*config.ServerDetails, error)
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
	reportUsage := usageReporter.ShouldReportUsage()
	if reportUsage {
		serverDetails, err := command.ServerDetails()
		if err != nil {
			log.Debug(usageReporter.ReportUsagePrefix, err.Error())
			return
		}
		if serverDetails != nil && serverDetails.ArtifactoryUrl != "" {
			log.Debug(usageReporter.ReportUsagePrefix, "Sending info...")
			serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
			if err != nil {
				log.Debug(usageReporter.ReportUsagePrefix, err.Error())
				return
			}
			err = usage.SendReportUsage(coreutils.GetCliUserAgent(), command.CommandName(), serviceManager)
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
