package common

import (
	"errors"
	"fmt"
	"os"

	cliUtils "github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
)

// Get the common 'server-id' flag
func GetServerIdFlag() components.StringFlag {
	return components.NewStringFlag("server-id", "Server ID configured using the config command.")
}

// Return the Artifactory Details of the provided 'server-id', or the default one.
func GetServerDetails(c *components.Context) (*config.ServerDetails, error) {
	details, err := commands.GetConfig(c.GetStringFlagValue("server-id"), false)
	if err != nil {
		return nil, err
	}
	if details.Url == "" {
		return nil, errors.New("no server-id was found, or the server-id has no url")
	}
	details.Url = clientUtils.AddTrailingSlashIfNeeded(details.Url)
	err = config.CreateInitialRefreshableTokensIfNeeded(details)
	if err != nil {
		return nil, err
	}
	return details, nil
}

func CreateServerDetailsFromFlags(c *components.Context) (details *config.ServerDetails, err error) {
	details = new(config.ServerDetails)
	details.Url = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("url"))
	details.ArtifactoryUrl = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("artifactory-url"))
	details.DistributionUrl = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("distribution-url"))
	details.XrayUrl = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("xray-url"))
	details.MissionControlUrl = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("mission-control-url"))
	details.PipelinesUrl = clientUtils.AddTrailingSlashIfNeeded(c.GetStringFlagValue("pipelines-url"))
	details.User = c.GetStringFlagValue("user")
	details.Password, err = HandleSecretInput(c, "password", "password-stdin")
	if err != nil {
		return
	}
	details.AccessToken, err = HandleSecretInput(c, "access-token", "access-token-stdin")
	if err != nil {
		return
	}
	details.SshKeyPath = c.GetStringFlagValue("ssh-key-path")
	details.SshPassphrase = c.GetStringFlagValue("ssh-passphrase")
	details.ClientCertPath = c.GetStringFlagValue("client-cert-path")
	details.ClientCertKeyPath = c.GetStringFlagValue("client-cert-key-path")
	details.ServerId = c.GetStringFlagValue("server-id")
	if details.ServerId == "" {
		details.ServerId = os.Getenv(coreutils.ServerID)
	}
	details.InsecureTls = c.GetBoolFlagValue("insecure-tls")
	return
}

func createServerDetailsFromFlags(c *components.Context, domain cliUtils.CommandDomain) (details *config.ServerDetails, err error) {
	details, err = CreateServerDetailsFromFlags(c)
	if err != nil {
		return
	}
	switch domain {
	case cliUtils.Rt:
		details.ArtifactoryUrl = details.Url
	case cliUtils.Xr:
		details.XrayUrl = details.Url
	case cliUtils.Ds:
		details.DistributionUrl = details.Url
	case cliUtils.Platform:
		return
	}
	details.Url = ""

	return
}

// Exclude refreshable tokens parameter should be true when working with external tools (build tools, curl, etc)
// or when sending requests not via ArtifactoryHttpClient.
func CreateServerDetailsWithConfigOffer(c *components.Context, excludeRefreshableTokens bool, domain cliUtils.CommandDomain) (*config.ServerDetails, error) {
	createdDetails, err := offerConfig(c, domain)
	if err != nil {
		return nil, err
	}
	if createdDetails != nil {
		return createdDetails, err
	}

	details, err := createServerDetailsFromFlags(c, domain)
	if err != nil {
		return nil, err
	}
	// If urls or credentials were passed as options, use options as they are.
	// For security reasons, we'd like to avoid using part of the connection details from command options and the rest from the config.
	// Either use command options only or config only.
	if credentialsChanged(details) {
		return details, nil
	}

	// Else, use details from config for requested serverId, or for default server if empty.
	confDetails, err := commands.GetConfig(details.ServerId, excludeRefreshableTokens)
	if err != nil {
		return nil, err
	}

	// Take insecureTls value from options since it is not saved in config.
	confDetails.InsecureTls = details.InsecureTls
	confDetails.Url = clientUtils.AddTrailingSlashIfNeeded(confDetails.Url)
	confDetails.DistributionUrl = clientUtils.AddTrailingSlashIfNeeded(confDetails.DistributionUrl)

	// Create initial access token if needed.
	if !excludeRefreshableTokens {
		err = config.CreateInitialRefreshableTokensIfNeeded(confDetails)
		if err != nil {
			return nil, err
		}
	}

	return confDetails, nil
}

func offerConfig(c *components.Context, domain cliUtils.CommandDomain) (*config.ServerDetails, error) {
	confirmed, err := ShouldOfferConfig()
	if !confirmed || err != nil {
		return nil, err
	}
	details, err := createServerDetailsFromFlags(c, domain)
	if err != nil {
		return nil, err
	}
	configCmd := commands.NewConfigCommand(commands.AddOrEdit, details.ServerId).SetDefaultDetails(details).SetInteractive(true).SetEncPassword(true)
	err = configCmd.Run()
	if err != nil {
		return nil, err
	}

	return configCmd.ServerDetails()
}

func ShouldOfferConfig() (bool, error) {
	exists, err := config.IsServerConfExists()
	if err != nil || exists {
		return false, err
	}
	clearConfigCmd := commands.NewConfigCommand(commands.Clear, "")
	var ci bool
	if ci, err = clientUtils.GetBoolEnvValue(coreutils.CI, false); err != nil {
		return false, err
	}
	if ci {
		_ = clearConfigCmd.Run()
		return false, nil
	}

	msg := fmt.Sprintf("To avoid this message in the future, set the %s environment variable to true.\n"+
		"The CLI commands require the URL and authentication details\n"+
		"Configuring JFrog CLI with these parameters now will save you having to include them as command options.\n"+
		"You can also configure these parameters later using the 'jfrog c' command.\n"+
		"Configure now?", coreutils.CI)
	confirmed := coreutils.AskYesNo(msg, false)
	if !confirmed {
		_ = clearConfigCmd.Run()
		return false, nil
	}
	return true, nil
}

func credentialsChanged(details *config.ServerDetails) bool {
	return details.Url != "" || details.ArtifactoryUrl != "" || details.DistributionUrl != "" || details.XrayUrl != "" ||
		details.User != "" || details.Password != "" || details.SshKeyPath != "" || details.SshPassphrase != "" || details.AccessToken != "" ||
		details.ClientCertKeyPath != "" || details.ClientCertPath != ""
}
