package commands

import (
	"sync"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreusage "github.com/jfrog/jfrog-cli-core/v2/utils/usage"
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
	// When the usage reporting is donw, signal to the channel.
	defer signalReportUsageFinished(channel)

	if !usageReporter.ShouldReportUsage() {
		log.Debug("Usage reporting is disabled")
		return
	}

	serverDetails, err := command.ServerDetails()
	if err != nil {
		log.Debug("Usage reporting:", err.Error())
		return
	}

	if serverDetails != nil {
		var wg sync.WaitGroup

		// Report the usage to Artifactory's Call Home API.
		if serverDetails.ArtifactoryUrl != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				reportUsageToArtifactoryCallHome(command, serverDetails)
			}()
		}

		// Repotr the usage to the Visibility System.
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToVisibilitySystem(command, serverDetails)
		}()

		// Wait for the two report actions to finish.
		wg.Wait()
	}
}

func reportUsageToVisibilitySystem(command Command, serverDetails *config.ServerDetails) {
	if err := coreusage.NewVisibilitySystemManager(serverDetails).SendUsage(command.CommandName()); err != nil {
		log.Debug("Visibility System Usage reporting:", err.Error())
	}
}

func reportUsageToArtifactoryCallHome(command Command, serverDetails *config.ServerDetails) {
	log.Debug(usageReporter.ArtifactoryCallHomePrefix, "Sending info...")
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		log.Debug(usageReporter.ArtifactoryCallHomePrefix, err.Error())
		return
	}
	if err = usage.NewArtifactoryCallHome().SendUsage(coreutils.GetCliUserAgent(), command.CommandName(), serviceManager); err != nil {
		log.Debug(err.Error())
	}
}

// Set to true when the report usage func exits
func signalReportUsageFinished(ch chan<- bool) {
	if ch != nil {
		ch <- true
	}
}
