package common

import (
	"errors"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
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
	return cliutils.CreateServerDetailsWithConfigOffer(func() (*config.ServerDetails, error) { return createServerDetailsFromFlags(c, domain) }, excludeRefreshableTokens)
}