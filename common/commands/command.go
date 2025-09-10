package commands

import (
	"sync"

	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	usageReporter "github.com/jfrog/jfrog-cli-core/v2/utils/usage"
	"github.com/jfrog/jfrog-cli-core/v2/utils/usage/visibility"
	rtClient "github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
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

// Exec executes a command and collects enhanced metrics
func Exec(command Command) error {
	commandName := command.CommandName()
	flags := GetContextFlags()

	log.Debug("Exec() collecting metrics for command:", commandName, "with flags:", flags)
	CollectMetrics(commandName, flags)

	channel := make(chan bool)
	go reportUsage(command, channel)
	err := command.Run()
	<-channel
	return err
}

// ExecAndThenReportUsage runs the command and then triggers a usage report.
// Used for commands which don't have the full server details before execution.
func ExecAndThenReportUsage(cc Command) (err error) {
	commandName := cc.CommandName()
	flags := GetContextFlags()

	log.Debug("ExecAndThenReportUsage() collecting metrics for command:", commandName, "with flags:", flags)
	CollectMetrics(commandName, flags)

	if err = cc.Run(); err != nil {
		return
	}
	reportUsage(cc, nil)
	return
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
	artifactoryVersion, err := serviceManager.GetVersion()
	if err != nil {
		log.Debug("Usage reporting. Failed getting the version of Artifactory", err.Error())
		return
	}

	var wg sync.WaitGroup

	// Report the usage to Artifactory's Call Home API.
	if version.NewVersion(artifactoryVersion).AtLeast(minCallHomeArtifactoryVersion) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToArtifactoryCallHome(command, serviceManager)
		}()
	}

	// Report the usage to the Visibility System.
	if version.NewVersion(artifactoryVersion).AtLeast(minVisibilitySystemArtifactoryVersion) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToVisibilitySystem(command, serverDetails)
		}()
	}

	// Wait for the two report actions to finish.
	wg.Wait()
}

// reportUsageToVisibilitySystem sends enhanced metrics to the visibility system
func reportUsageToVisibilitySystem(command Command, serverDetails *config.ServerDetails) {
	var commandsCountMetric services.VisibilityMetric

	commandName := command.CommandName()
	metricsData := GetCollectedMetrics(commandName)

	if metricsData != nil {
		log.Debug("Using enhanced metrics for command:", commandName)
		visibilityMetricsData := &visibility.MetricsData{
			FlagsUsed:    metricsData.FlagsUsed,
			OS:           metricsData.OS,
			Architecture: metricsData.Architecture,
			IsCI:         metricsData.IsCI,
			CISystem:     metricsData.CISystem,
			IsContainer:  metricsData.IsContainer,
		}
		commandsCountMetric = visibility.NewCommandsCountMetricWithEnhancedData(commandName, visibilityMetricsData)
		ClearCollectedMetrics(commandName)
	} else {
		log.Debug("No enhanced metrics found for command:", commandName, "- using standard metric")
		commandsCountMetric = visibility.NewCommandsCountMetric(commandName)
	}

	if err := visibility.NewVisibilitySystemManager(serverDetails).SendUsage(commandsCountMetric); err != nil {
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
