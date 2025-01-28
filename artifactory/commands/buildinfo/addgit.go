package buildinfo

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"strconv"

	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	utilsconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"

	"github.com/forPelevin/gomoji"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const (
	GitLogLimit               = 100
	ConfigIssuesPrefix        = "issues."
	ConfigParseValueError     = "Failed parsing %s from configuration file: %s"
	MissingConfigurationError = "Configuration file must contain: %s"
	gitParsingPrettyFormat    = "format:%s"
)

type BuildAddGitCommand struct {
	buildConfiguration *build.BuildConfiguration
	dotGitPath         string
	configFilePath     string
	serverId           string
	issuesConfig       *IssuesConfiguration
}

func NewBuildAddGitCommand() *BuildAddGitCommand {
	return &BuildAddGitCommand{}
}

func (config *BuildAddGitCommand) SetIssuesConfig(issuesConfig *IssuesConfiguration) *BuildAddGitCommand {
	config.issuesConfig = issuesConfig
	return config
}

func (config *BuildAddGitCommand) SetConfigFilePath(configFilePath string) *BuildAddGitCommand {
	config.configFilePath = configFilePath
	return config
}

func (config *BuildAddGitCommand) SetDotGitPath(dotGitPath string) *BuildAddGitCommand {
	config.dotGitPath = dotGitPath
	return config
}

func (config *BuildAddGitCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildAddGitCommand {
	config.buildConfiguration = buildConfiguration
	return config
}

func (config *BuildAddGitCommand) SetServerId(serverId string) *BuildAddGitCommand {
	config.serverId = serverId
	return config
}

func (config *BuildAddGitCommand) Run() error {
	log.Info("Reading the git branch, revision and remote URL and adding them to the build-info.")
	buildName, err := config.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := config.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	err = build.SaveBuildGeneralDetails(buildName, buildNumber, config.buildConfiguration.GetProject())
	if err != nil {
		return err
	}

	// Find .git if it wasn't provided in the command.
	config.dotGitPath, err = utils.GetDotGit(config.dotGitPath)
	if err != nil {
		return err
	}

	// Collect URL, branch and revision into GitManager.
	gitManager := clientutils.NewGitManager(config.dotGitPath)
	err = gitManager.ReadConfig()
	if err != nil {
		return err
	}

	// Collect issues if required.
	var issues []buildinfo.AffectedIssue
	if config.configFilePath != "" {
		issues, err = config.collectBuildIssues()
		if err != nil {
			return err
		}
	}

	// Populate partials with VCS info.
	populateFunc := func(partial *buildinfo.Partial) {
		partial.VcsList = append(partial.VcsList, buildinfo.Vcs{
			Url:      gitManager.GetUrl(),
			Revision: gitManager.GetRevision(),
			Branch:   gitManager.GetBranch(),
			Message:  gomoji.RemoveEmojis(gitManager.GetMessage()),
		})

		if config.configFilePath != "" {
			partial.Issues = &buildinfo.Issues{
				Tracker:                &buildinfo.Tracker{Name: config.issuesConfig.TrackerName, Version: ""},
				AggregateBuildIssues:   config.issuesConfig.Aggregate,
				AggregationBuildStatus: config.issuesConfig.AggregationStatus,
				AffectedIssues:         issues,
			}
		}
	}
	err = build.SavePartialBuildInfo(buildName, buildNumber, config.buildConfiguration.GetProject(), populateFunc)
	if err != nil {
		return err
	}

	// Done.
	log.Debug("Collected VCS details for", buildName+"/"+buildNumber+".")
	return nil
}

// Priorities for selecting server:
// 1. 'server-id' flag.
// 2. 'serverID' in config file.
// 3. Default server.
func (config *BuildAddGitCommand) ServerDetails() (*utilsconfig.ServerDetails, error) {
	var serverId string
	if config.serverId != "" {
		serverId = config.serverId
	} else if config.configFilePath != "" {
		// Get the server ID from the conf file.
		var vConfig *viper.Viper
		vConfig, err := project.ReadConfigFile(config.configFilePath, project.YAML)
		if err != nil {
			return nil, err
		}
		serverId = vConfig.GetString(ConfigIssuesPrefix + "serverID")
	}
	return utilsconfig.GetSpecificConfig(serverId, true, false)
}

func (config *BuildAddGitCommand) CommandName() string {
	return "rt_build_add_git"
}

func (config *BuildAddGitCommand) collectBuildIssues() ([]buildinfo.AffectedIssue, error) {
	log.Info("Collecting build issues from VCS...")

	// Initialize issues-configuration.
	config.issuesConfig = new(IssuesConfiguration)

	// Create config's IssuesConfigurations from the provided spec file.
	err := config.createIssuesConfigs()
	if err != nil {
		return nil, err
	}

	var foundIssues []buildinfo.AffectedIssue
	logRegExp, err := createLogRegExpHandler(config.issuesConfig, &foundIssues)
	if err != nil {
		return nil, err
	}

	// Run issues collection.
	gitDetails := utils.GitParsingDetails{DotGitPath: config.dotGitPath, LogLimit: config.issuesConfig.LogLimit, PrettyFormat: gitParsingPrettyFormat}
	err = utils.ParseGitLogFromLastBuild(config.issuesConfig.ServerDetails, config.buildConfiguration, gitDetails, logRegExp)
	if err != nil {
		return nil, err
	}
	return foundIssues, nil
}

// Creates a regexp handler to parse and fetch issues from the output of the git log command.
func createLogRegExpHandler(issuesConfig *IssuesConfiguration, foundIssues *[]buildinfo.AffectedIssue) (*gofrogcmd.CmdOutputPattern, error) {
	// Create regex pattern.
	issueRegexp, err := clientutils.GetRegExp(issuesConfig.Regexp)
	if err != nil {
		return nil, err
	}

	// Create handler with exec function.
	logRegExp := gofrogcmd.CmdOutputPattern{
		RegExp: issueRegexp,
		ExecFunc: func(pattern *gofrogcmd.CmdOutputPattern) (string, error) {
			// Reached here - means no error occurred.

			// Check for out of bound results.
			if len(pattern.MatchedResults)-1 < issuesConfig.KeyGroupIndex || len(pattern.MatchedResults)-1 < issuesConfig.SummaryGroupIndex {
				return "", errors.New("unexpected result while parsing issues from git log. Make sure that the regular expression used to find issues, includes two capturing groups, for the issue ID and the summary")
			}
			// Create found Affected Issue.
			foundIssue := buildinfo.AffectedIssue{Key: pattern.MatchedResults[issuesConfig.KeyGroupIndex], Summary: pattern.MatchedResults[issuesConfig.SummaryGroupIndex], Aggregated: false}
			if issuesConfig.TrackerUrl != "" {
				foundIssue.Url = issuesConfig.TrackerUrl + pattern.MatchedResults[issuesConfig.KeyGroupIndex]
			}
			*foundIssues = append(*foundIssues, foundIssue)
			log.Debug("Found issue: " + pattern.MatchedResults[issuesConfig.KeyGroupIndex])
			return "", nil
		},
	}
	return &logRegExp, nil
}

func (config *BuildAddGitCommand) createIssuesConfigs() (err error) {
	// Read file's data.
	err = config.issuesConfig.populateIssuesConfigsFromSpec(config.configFilePath)
	if err != nil {
		return
	}

	// Use 'server-id' flag if provided.
	if config.serverId != "" {
		config.issuesConfig.ServerID = config.serverId
	}

	// Build ServerDetails from provided serverID.
	err = config.issuesConfig.setServerDetails()
	if err != nil {
		return
	}

	// Add '/' suffix to URL if required.
	if config.issuesConfig.TrackerUrl != "" {
		// Url should end with '/'
		config.issuesConfig.TrackerUrl = clientutils.AddTrailingSlashIfNeeded(config.issuesConfig.TrackerUrl)
	}

	return
}

func (ic *IssuesConfiguration) populateIssuesConfigsFromSpec(configFilePath string) (err error) {
	var vConfig *viper.Viper
	vConfig, err = project.ReadConfigFile(configFilePath, project.YAML)
	if err != nil {
		return err
	}

	// Validate that the config contains issues.
	if !vConfig.IsSet("issues") {
		return errorutils.CheckErrorf(MissingConfigurationError, "issues")
	}

	// Get server-id.
	if vConfig.IsSet(ConfigIssuesPrefix + "serverID") {
		ic.ServerID = vConfig.GetString(ConfigIssuesPrefix + "serverID")
	}

	// Set log limit.
	ic.LogLimit = GitLogLimit

	// Get tracker data
	if !vConfig.IsSet(ConfigIssuesPrefix + "trackerName") {
		return errorutils.CheckErrorf(MissingConfigurationError, ConfigIssuesPrefix+"trackerName")
	}
	ic.TrackerName = vConfig.GetString(ConfigIssuesPrefix + "trackerName")

	// Get issues pattern
	if !vConfig.IsSet(ConfigIssuesPrefix + "regexp") {
		return errorutils.CheckErrorf(MissingConfigurationError, ConfigIssuesPrefix+"regexp")
	}
	ic.Regexp = vConfig.GetString(ConfigIssuesPrefix + "regexp")

	// Get issues base url
	if vConfig.IsSet(ConfigIssuesPrefix + "trackerUrl") {
		ic.TrackerUrl = vConfig.GetString(ConfigIssuesPrefix + "trackerUrl")
	}

	// Get issues key group index
	if !vConfig.IsSet(ConfigIssuesPrefix + "keyGroupIndex") {
		return errorutils.CheckErrorf(MissingConfigurationError, ConfigIssuesPrefix+"keyGroupIndex")
	}
	ic.KeyGroupIndex, err = strconv.Atoi(vConfig.GetString(ConfigIssuesPrefix + "keyGroupIndex"))
	if err != nil {
		return errorutils.CheckErrorf(ConfigParseValueError, ConfigIssuesPrefix+"keyGroupIndex", err.Error())
	}

	// Get issues summary group index
	if !vConfig.IsSet(ConfigIssuesPrefix + "summaryGroupIndex") {
		return errorutils.CheckErrorf(MissingConfigurationError, ConfigIssuesPrefix+"summaryGroupIndex")
	}
	ic.SummaryGroupIndex, err = strconv.Atoi(vConfig.GetString(ConfigIssuesPrefix + "summaryGroupIndex"))
	if err != nil {
		return errorutils.CheckErrorf(ConfigParseValueError, ConfigIssuesPrefix+"summaryGroupIndex", err.Error())
	}

	// Get aggregation aggregate
	ic.Aggregate = false
	if vConfig.IsSet(ConfigIssuesPrefix + "aggregate") {
		ic.Aggregate, err = strconv.ParseBool(vConfig.GetString(ConfigIssuesPrefix + "aggregate"))
		if err != nil {
			return errorutils.CheckErrorf(ConfigParseValueError, ConfigIssuesPrefix+"aggregate", err.Error())
		}
	}

	// Get aggregation status
	if vConfig.IsSet(ConfigIssuesPrefix + "aggregationStatus") {
		ic.AggregationStatus = vConfig.GetString(ConfigIssuesPrefix + "aggregationStatus")
	}

	return nil
}

func (ic *IssuesConfiguration) setServerDetails() error {
	// If no server-id provided, use default server.
	serverDetails, err := utilsconfig.GetSpecificConfig(ic.ServerID, true, false)
	if err != nil {
		return err
	}
	ic.ServerDetails = serverDetails
	return nil
}

type IssuesConfiguration struct {
	ServerDetails     *utilsconfig.ServerDetails
	Regexp            string
	LogLimit          int
	TrackerUrl        string
	TrackerName       string
	KeyGroupIndex     int
	SummaryGroupIndex int
	Aggregate         bool
	AggregationStatus string
	ServerID          string
}
