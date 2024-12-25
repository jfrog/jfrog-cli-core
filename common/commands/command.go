package commands

import (
	"sync"

	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreusage "github.com/jfrog/jfrog-cli-core/v2/utils/usage"
	usageReporter "github.com/jfrog/jfrog-cli-core/v2/utils/usage"
	rtClient "github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	minCallHomeArtifactoryVersion         = "6.9.0"
	minVisibilitySystemArtifactoryVersion = "7.102"
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
	// When the usage reporting is done, signal to the channel.
	defer signalReportUsageFinished(channel)

	if !usageReporter.ShouldReportUsage() {
		log.Debug("Usage reporting is disabled")
		return
	}

	serverDetails, err := command.ServerDetails()
	if err != nil {
		log.Debug("Usage reporting. Failed accessing ServerDetails.", err.Error())
		return
	}
	if serverDetails == nil || serverDetails.ArtifactoryUrl == "" {
		return
	}
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		log.Debug("Usage reporting. Failed creating the Artifactory Service Manager.", err.Error())
		return
	}
	ArtifactoryVersion, err := serviceManager.GetVersion()
	if err != nil {
		log.Debug("Usage reporting. Failed getting the Artifactory", err.Error())
		return
	}

	var wg sync.WaitGroup

	// Report the usage to Artifactory's Call Home API.
	if version.NewVersion(ArtifactoryVersion).AtLeast(minCallHomeArtifactoryVersion) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToArtifactoryCallHome(command, serviceManager)
		}()
	}

	// Report the usage to the Visibility System.
	if version.NewVersion(ArtifactoryVersion).AtLeast(minVisibilitySystemArtifactoryVersion) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToVisibilitySystem(command, serverDetails)
		}()
	}

	// Wait for the two report actions to finish.
	wg.Wait()
}

func reportUsageToVisibilitySystem(command Command, serverDetails *config.ServerDetails) {
	if err := coreusage.NewVisibilitySystemManager(serverDetails).SendUsage(command.CommandName()); err != nil {
		log.Debug("Visibility System Usage reporting:", err.Error())
	}
}

func reportUsageToArtifactoryCallHome(command Command, serviceManager rtClient.ArtifactoryServicesManager) {
	log.Debug(usageReporter.ArtifactoryCallHomePrefix, "Sending info...")
	if err := usage.NewArtifactoryCallHome().Send(coreutils.GetCliUserAgent(), command.CommandName(), serviceManager); err != nil {
		log.Debug(err.Error())
	}
}

// Set to true when the report usage func exits
func signalReportUsageFinished(ch chan<- bool) {
	if ch != nil {
		ch <- true
	}
}
