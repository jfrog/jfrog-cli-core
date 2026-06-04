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

func Exec(command Command) error {
	commandName := command.CommandName()
	flags := GetContextFlags()
	CollectMetrics(commandName, flags)
	channel := make(chan bool)
	// Triggers the report usage.
	go reportCommandUsage(command, channel)
	// Invoke the command interface
	err := command.Run()
	// Waits for the signal from the report usage to be done.
	<-channel
	return err
}

// ExecWithPackageManager tags the command with the given package manager
// name (e.g. "npm", "go", "docker") for telemetry, then runs Exec.
// The package manager context is consumed by CollectMetrics inside Exec.
func ExecWithPackageManager(command Command, packageManager string) error {
	SetPackageManagerContext(packageManager)
	return Exec(command)
}

// ExecAndThenReportUsage runs the command and then triggers a usage report.
// Used for commands which don't have the full server details before execution.
// For example: oidc exchange command, which will get access token only after execution.
func ExecAndThenReportUsage(cc Command) (err error) {
	commandName := cc.CommandName()
	flags := GetContextFlags()
	CollectMetrics(commandName, flags)
	if err = cc.Run(); err != nil {
		return
	}
	reportCommandUsage(cc, nil)
	return
}

func reportCommandUsage(command Command, channel chan<- bool) {
	commandName := command.CommandName()
	serverDetails, err := command.ServerDetails()
	if err != nil {
		log.Debug("Usage reporting. Failed accessing ServerDetails.", err.Error())
		signalReportUsageFinished(channel)
		return
	}
	ReportUsage(commandName, serverDetails, channel)
}

// ReportUsage sends a usage report for the given command to the JFrog platform.
// It reports to two destinations in parallel, depending on the Artifactory version:
//   - Artifactory's Call Home API (requires Artifactory >= minCallHomeArtifactoryVersion).
//   - The Visibility System (requires Artifactory >= minVisibilitySystemArtifactoryVersion),
//     which receives enhanced metrics collected via CollectMetrics.
//
// The function is a no-op when usage reporting is disabled (see usageReporter.ShouldReportUsage)
// or when serverDetails is nil or has no ArtifactoryUrl.
//
// If channel is non-nil, a value is sent on it once reporting completes (including early returns),
// allowing callers to run reporting asynchronously and wait for completion.
func ReportUsage(commandName string, serverDetails *config.ServerDetails, channel chan<- bool) {
	// When the usage reporting is done, signal to the channel.
	defer signalReportUsageFinished(channel)

	if !usageReporter.ShouldReportUsage() {
		log.Debug("Usage reporting is disabled")
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
			reportUsageToArtifactoryCallHome(commandName, serviceManager)
		}()
	}

	// Report the usage to the Visibility System.
	if version.NewVersion(artifactoryVersion).AtLeast(minVisibilitySystemArtifactoryVersion) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reportUsageToVisibilitySystem(commandName, serverDetails)
		}()
	}

	// Wait for the two report actions to finish.
	wg.Wait()
}

// reportUsageToVisibilitySystem sends enhanced metrics to the visibility system
func reportUsageToVisibilitySystem(commandName string, serverDetails *config.ServerDetails) {
	var commandsCountMetric services.VisibilityMetric

	metricsData := GetCollectedMetrics(commandName)
	var visibilityMetricsData *visibility.MetricsData
	if metricsData != nil {
		visibilityMetricsData = &visibility.MetricsData{
			Flags:          metricsData.Flags,
			Platform:       metricsData.Platform,
			Architecture:   metricsData.Architecture,
			IsCI:           metricsData.IsCI,
			CISystem:       metricsData.CISystem,
			IsContainer:    metricsData.IsContainer,
			IsAgent:        metricsData.IsAgent,
			Agent:          metricsData.Agent,
			IsInteractive:  metricsData.IsInteractive,
			PackageAlias:   metricsData.PackageAlias,
			PackageManager: metricsData.PackageManager,
		}
	}
	commandsCountMetric = visibility.NewCommandsCountMetricWithEnhancedData(commandName, visibilityMetricsData)

	if err := visibility.NewVisibilitySystemManager(serverDetails).SendUsage(commandsCountMetric); err != nil {
		log.Debug("Visibility System Usage reporting:", err.Error())
	}
}

func reportUsageToArtifactoryCallHome(commandName string, serviceManager rtClient.ArtifactoryServicesManager) {
	log.Debug(usageReporter.ArtifactoryCallHomePrefix, "Sending info...")
	if err := usage.NewArtifactoryCallHome().Send(coreutils.GetCliUserAgent(), commandName, serviceManager); err != nil {
		log.Debug(err.Error())
	}
}

// Set to true when the report usage func exits
func signalReportUsageFinished(ch chan<- bool) {
	if ch != nil {
		ch <- true
	}
}
