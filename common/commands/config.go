package commands

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"

	"github.com/jfrog/jfrog-client-go/auth"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Internal golang locking for the same process.
var mutex sync.Mutex

type ConfigCommand struct {
	details          *config.ServerDetails
	defaultDetails   *config.ServerDetails
	interactive      bool
	encPassword      bool
	useBasicAuthOnly bool
	serverId         string
	// For unit tests
	disablePromptUrls bool
}

func NewConfigCommand() *ConfigCommand {
	return &ConfigCommand{}
}

func (cc *ConfigCommand) SetServerId(serverId string) *ConfigCommand {
	cc.serverId = serverId
	return cc
}

func (cc *ConfigCommand) SetEncPassword(encPassword bool) *ConfigCommand {
	cc.encPassword = encPassword
	return cc
}

func (cc *ConfigCommand) SetUseBasicAuthOnly(useBasicAuthOnly bool) *ConfigCommand {
	cc.useBasicAuthOnly = useBasicAuthOnly
	return cc
}

func (cc *ConfigCommand) SetInteractive(interactive bool) *ConfigCommand {
	cc.interactive = interactive
	return cc
}

func (cc *ConfigCommand) SetDefaultDetails(defaultDetails *config.ServerDetails) *ConfigCommand {
	cc.defaultDetails = defaultDetails
	return cc
}

func (cc *ConfigCommand) SetDetails(details *config.ServerDetails) *ConfigCommand {
	cc.details = details
	return cc
}

func (cc *ConfigCommand) Run() error {
	return cc.Config()
}

func (cc *ConfigCommand) ServerDetails() (*config.ServerDetails, error) {
	// If cc.details is not empty, then return it.
	if cc.details != nil && !reflect.DeepEqual(config.ServerDetails{}, *cc.details) {
		return cc.details, nil
	}
	// If cc.defaultDetails is not empty, then return it.
	if cc.defaultDetails != nil && !reflect.DeepEqual(config.ServerDetails{}, *cc.defaultDetails) {
		return cc.defaultDetails, nil
	}
	return nil, nil
}

func (cc *ConfigCommand) CommandName() string {
	return "config"
}

func (cc *ConfigCommand) Config() error {
	mutex.Lock()
	defer mutex.Unlock()
	lockDirPath, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return err
	}
	lockFile, err := lock.CreateLock(lockDirPath)
	defer lockFile.Unlock()

	if err != nil {
		return err
	}

	configurations, err := cc.prepareConfigurationData()
	if err != nil {
		return err
	}
	if cc.interactive {
		err = cc.getConfigurationFromUser()
		if err != nil {
			return err
		}
	} else if cc.details.Url != "" {
		if fileutils.IsSshUrl(cc.details.Url) {
			coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url)
		} else {
			cc.details.Url = clientutils.AddTrailingSlashIfNeeded(cc.details.Url)
			// Derive JFrog services URLs from platform URL
			coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url+"artifactory/")
			coreutils.SetIfEmpty(&cc.details.DistributionUrl, cc.details.Url+"distribution/")
			coreutils.SetIfEmpty(&cc.details.XrayUrl, cc.details.Url+"xray/")
			coreutils.SetIfEmpty(&cc.details.MissionControlUrl, cc.details.Url+"mc/")
			coreutils.SetIfEmpty(&cc.details.PipelinesUrl, cc.details.Url+"pipelines/")
		}
	}
	cc.details.ArtifactoryUrl = clientutils.AddTrailingSlashIfNeeded(cc.details.ArtifactoryUrl)
	cc.details.DistributionUrl = clientutils.AddTrailingSlashIfNeeded(cc.details.DistributionUrl)
	cc.details.XrayUrl = clientutils.AddTrailingSlashIfNeeded(cc.details.XrayUrl)
	cc.details.MissionControlUrl = clientutils.AddTrailingSlashIfNeeded(cc.details.MissionControlUrl)
	cc.details.PipelinesUrl = clientutils.AddTrailingSlashIfNeeded(cc.details.PipelinesUrl)

	// Artifactory expects the username to be lower-cased. In case it is not,
	// Artifactory will silently save it lower-cased, but the token creation
	// REST API will fail with a non lower-cased username.
	cc.details.User = strings.ToLower(cc.details.User)

	if len(configurations) == 1 {
		cc.details.IsDefault = true
	}

	err = checkSingleAuthMethod(cc.details)
	if err != nil {
		return err
	}

	if cc.encPassword && cc.details.ArtifactoryUrl != "" {
		err = cc.encryptPassword()
		if err != nil {
			return errorutils.CheckErrorf("The following error was received while trying to encrypt your password: %s ", err)
		}
	}

	if !cc.useBasicAuthOnly {
		cc.configRefreshableToken()
	}

	return config.SaveServersConf(configurations)
}

func (cc *ConfigCommand) configRefreshableToken() {
	if cc.details.User == "" || cc.details.Password == "" {
		return
	}
	// Set the default interval for the refreshable tokens to be initialized in the next CLI run.
	cc.details.TokenRefreshInterval = coreutils.TokenRefreshDefaultInterval
}

func (cc *ConfigCommand) prepareConfigurationData() ([]*config.ServerDetails, error) {
	// If details is nil, initialize a new one
	if cc.details == nil {
		cc.details = new(config.ServerDetails)
		if cc.defaultDetails != nil {
			cc.details.InsecureTls = cc.defaultDetails.InsecureTls
		}
	}

	// Get configurations list
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return configurations, err
	}

	// Get default server details
	if cc.defaultDetails == nil {
		cc.defaultDetails, err = config.GetDefaultConfiguredConf(configurations)
		if err != nil {
			return configurations, errorutils.CheckError(err)
		}
	}

	// Get server id
	if cc.interactive && cc.serverId == "" {
		ioutils.ScanFromConsole("Server ID", &cc.serverId, cc.defaultDetails.ServerId)
	}
	cc.details.ServerId = cc.resolveServerId()

	// Remove and get the server details from the configurations list
	tempConfiguration, configurations := config.GetAndRemoveConfiguration(cc.details.ServerId, configurations)

	// Change default server details if the server was exist in the configurations list
	if tempConfiguration != nil {
		cc.defaultDetails = tempConfiguration
		cc.details.IsDefault = tempConfiguration.IsDefault
	}

	// Append the configuration to the configurations list
	configurations = append(configurations, cc.details)
	return configurations, err
}

/// Returning the first non empty value:
// 1. The serverId argument sent.
// 2. details.ServerId
// 3. defaultDetails.ServerId
// 4. config.DEFAULT_SERVER_ID
func (cc *ConfigCommand) resolveServerId() string {
	if cc.serverId != "" {
		return cc.serverId
	}
	if cc.details.ServerId != "" {
		return cc.details.ServerId
	}
	if cc.defaultDetails.ServerId != "" {
		return cc.defaultDetails.ServerId
	}
	return config.DefaultServerId
}

func (cc *ConfigCommand) getConfigurationFromUser() error {
	disallowUsingSavedPassword := false

	if cc.details.Url == "" {
		ioutils.ScanFromConsole("JFrog platform URL", &cc.details.Url, cc.defaultDetails.Url)
	}

	if cc.details.Url != "" {
		if fileutils.IsSshUrl(cc.details.Url) {
			coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url)
		} else {
			cc.details.Url = clientutils.AddTrailingSlashIfNeeded(cc.details.Url)
			disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.DistributionUrl, cc.details.Url+"distribution/") || disallowUsingSavedPassword
			disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url+"artifactory/") || disallowUsingSavedPassword
			disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.XrayUrl, cc.details.Url+"xray/") || disallowUsingSavedPassword
			disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.MissionControlUrl, cc.details.Url+"mc/") || disallowUsingSavedPassword
			disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.PipelinesUrl, cc.details.Url+"pipelines/") || disallowUsingSavedPassword
		}
	}

	if fileutils.IsSshUrl(cc.details.ArtifactoryUrl) {
		if err := getSshKeyPath(cc.details); err != nil {
			return err
		}
	} else {
		if !cc.disablePromptUrls {
			if err := cc.promptUrls(&disallowUsingSavedPassword); err != nil {
				return err
			}
		}
		// Password/Access-Token
		if cc.details.Password == "" && cc.details.AccessToken == "" {
			err := readAccessTokenFromConsole(cc.details)
			if err != nil {
				return err
			}
			if len(cc.details.GetAccessToken()) == 0 {
				err = ioutils.ReadCredentialsFromConsole(cc.details, cc.defaultDetails, disallowUsingSavedPassword)
				if err != nil {
					return err
				}
			}
		}
	}

	cc.readClientCertInfoFromConsole()
	return nil
}

func (cc *ConfigCommand) promptUrls(disallowUsingSavedPassword *bool) error {
	promptItems := []ioutils.PromptItem{
		{Option: "JFrog Artifactory URL", TargetValue: &cc.details.ArtifactoryUrl, DefaultValue: cc.defaultDetails.ArtifactoryUrl},
		{Option: "JFrog Distribution URL", TargetValue: &cc.details.DistributionUrl, DefaultValue: cc.defaultDetails.DistributionUrl},
		{Option: "JFrog Xray URL", TargetValue: &cc.details.XrayUrl, DefaultValue: cc.defaultDetails.XrayUrl},
		{Option: "JFrog Mission Control URL", TargetValue: &cc.details.MissionControlUrl, DefaultValue: cc.defaultDetails.MissionControlUrl},
		{Option: "JFrog Pipelines URL", TargetValue: &cc.details.PipelinesUrl, DefaultValue: cc.defaultDetails.PipelinesUrl},
	}
	return ioutils.PromptStrings(promptItems, "Select 'Save and continue' or modify any of the URLs", func(item ioutils.PromptItem) {
		*disallowUsingSavedPassword = true
		ioutils.ScanFromConsole(item.Option, item.TargetValue, item.DefaultValue)
	})
}

func (cc *ConfigCommand) readClientCertInfoFromConsole() {
	if cc.details.ClientCertPath != "" && cc.details.ClientCertKeyPath != "" {
		return
	}
	if coreutils.AskYesNo("Is the Artifactory reverse proxy configured to accept a client certificate?", false) {
		if cc.details.ClientCertPath == "" {
			ioutils.ScanFromConsole("Client certificate file path", &cc.details.ClientCertPath, cc.defaultDetails.ClientCertPath)
		}
		if cc.details.ClientCertKeyPath == "" {
			ioutils.ScanFromConsole("Client certificate key path", &cc.details.ClientCertKeyPath, cc.defaultDetails.ClientCertKeyPath)
		}
	}
}

func (cc *ConfigCommand) readRefreshableTokenFromConsole() {
	if !cc.useBasicAuthOnly && (cc.details.Password != "" && cc.details.AccessToken == "") {
		useRefreshableToken := coreutils.AskYesNo("For commands which don't use external tools or the JFrog Distribution service, "+
			"JFrog CLI supports replacing the configured username and password/API key with automatically created access token that's refreshed hourly. "+
			"Enable this setting?", true)
		cc.useBasicAuthOnly = !useRefreshableToken
	}
	return
}

func readAccessTokenFromConsole(details *config.ServerDetails) error {
	token, err := ioutils.ScanPasswordFromConsole("JFrog access token (Leave blank for username and password/API key): ")
	if err == nil {
		details.SetAccessToken(token)
	}
	return err
}

func getSshKeyPath(details *config.ServerDetails) error {
	// If path not provided as a key, read from console:
	if details.SshKeyPath == "" {
		ioutils.ScanFromConsole("SSH key file path (optional)", &details.SshKeyPath, "")
	}

	// If path still not provided, return and warn about relying on agent.
	if details.SshKeyPath == "" {
		log.Info("SSH Key path not provided. The ssh-agent (if active) will be used.")
		return nil
	}

	// If SSH key path provided, check if exists:
	details.SshKeyPath = clientutils.ReplaceTildeWithUserHome(details.SshKeyPath)
	exists, err := fileutils.IsFileExists(details.SshKeyPath, false)
	if err != nil {
		return err
	}

	messageSuffix := ": "
	if exists {
		sshKeyBytes, err := ioutil.ReadFile(details.SshKeyPath)
		if err != nil {
			return nil
		}
		encryptedKey, err := auth.IsEncrypted(sshKeyBytes)
		// If exists and not encrypted (or error occurred), return without asking for passphrase
		if err != nil || !encryptedKey {
			return err
		}
		log.Info("The key file at the specified path is encrypted.")
	} else {
		log.Info("Could not find key in provided path. You may place the key file there later.")
		messageSuffix = " (optional): "
	}
	if details.SshPassphrase == "" {
		token, err := ioutils.ScanPasswordFromConsole("SSH key passphrase" + messageSuffix)
		if err != nil {
			return err
		}
		details.SetSshPassphrase(token)
	}
	return err
}

func ShowConfig(serverName string) error {
	var configuration []*config.ServerDetails
	if serverName != "" {
		singleConfig, err := config.GetSpecificConfig(serverName, true, false)
		if err != nil {
			return err
		}
		configuration = []*config.ServerDetails{singleConfig}
	} else {
		var err error
		configuration, err = config.GetAllServersConfigs()
		if err != nil {
			return err
		}
	}
	printConfigs(configuration)
	return nil
}

func Import(serverToken string) error {
	serverDetails, err := config.Import(serverToken)
	if err != nil {
		return err
	}
	log.Info("Importing server ID", "'"+serverDetails.ServerId+"'")
	configCommand := &ConfigCommand{
		details:  serverDetails,
		serverId: serverDetails.ServerId,
	}
	return configCommand.Config()
}

func Export(serverName string) error {
	serverDetails, err := config.GetSpecificConfig(serverName, true, false)
	if err != nil {
		return err
	}
	serverToken, err := config.Export(serverDetails)
	if err != nil {
		return err
	}
	log.Output(serverToken)
	return nil
}

func printConfigs(configuration []*config.ServerDetails) {
	for _, details := range configuration {
		logIfNotEmpty(details.ServerId, "Server ID:\t\t\t", false)
		logIfNotEmpty(details.Url, "JFrog platform URL:\t\t", false)
		logIfNotEmpty(details.ArtifactoryUrl, "Artifactory URL:\t\t", false)
		logIfNotEmpty(details.DistributionUrl, "Distribution URL:\t\t", false)
		logIfNotEmpty(details.XrayUrl, "Xray URL:\t\t\t", false)
		logIfNotEmpty(details.MissionControlUrl, "Mission Control URL:\t\t", false)
		logIfNotEmpty(details.PipelinesUrl, "Pipelines URL:\t\t\t", false)
		logIfNotEmpty(details.User, "User:\t\t\t\t", false)
		logIfNotEmpty(details.Password, "Password:\t\t\t", true)
		logIfNotEmpty(details.AccessToken, "Access token:\t\t\t", true)
		logIfNotEmpty(details.RefreshToken, "Refresh token:\t\t\t", true)
		logIfNotEmpty(details.SshKeyPath, "SSH key file path:\t\t", false)
		logIfNotEmpty(details.SshPassphrase, "SSH passphrase:\t\t\t", true)
		logIfNotEmpty(details.ClientCertPath, "Client certificate file path:\t", false)
		logIfNotEmpty(details.ClientCertKeyPath, "Client certificate key path:\t", false)
		log.Output("Default:\t\t\t" + strconv.FormatBool(details.IsDefault))
		log.Output()
	}
}

func logIfNotEmpty(value, prefix string, mask bool) {
	if value != "" {
		if mask {
			value = "***"
		}
		log.Output(prefix + value)
	}
}

func DeleteConfig(serverName string) error {
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	var isDefault, isFoundName bool
	for i, config := range configurations {
		if config.ServerId == serverName {
			isDefault = config.IsDefault
			configurations = append(configurations[:i], configurations[i+1:]...)
			isFoundName = true
			break
		}

	}
	if isDefault && len(configurations) > 0 {
		configurations[0].IsDefault = true
	}
	if isFoundName {
		return config.SaveServersConf(configurations)
	}
	log.Info("\"" + serverName + "\" configuration could not be found.\n")
	return nil
}

// Set the default configuration
func Use(serverId string) error {
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	var serverFound *config.ServerDetails
	newDefaultServer := true
	for _, config := range configurations {
		if config.ServerId == serverId {
			serverFound = config
			if config.IsDefault {
				newDefaultServer = false
				break
			}
			config.IsDefault = true
		} else {
			config.IsDefault = false
		}
	}
	// Need to save only if we found a server with the serverId
	if serverFound != nil {
		if newDefaultServer {
			err = config.SaveServersConf(configurations)
			if err != nil {
				return err
			}
		}
		log.Info(fmt.Sprintf("Using server ID '%s' (%s).", serverFound.ServerId, serverFound.Url))
		return nil
	}
	return errorutils.CheckErrorf("Could not find a server with ID '%s'.", serverId)
}

func ClearConfig(interactive bool) {
	if interactive {
		confirmed := coreutils.AskYesNo("Are you sure you want to delete all the configurations?", false)
		if !confirmed {
			return
		}
	}
	config.SaveServersConf(make([]*config.ServerDetails, 0))
}

func GetConfig(serverId string, excludeRefreshableTokens bool) (*config.ServerDetails, error) {
	return config.GetSpecificConfig(serverId, true, excludeRefreshableTokens)
}

func (cc *ConfigCommand) encryptPassword() error {
	if cc.details.Password == "" {
		return nil
	}

	artAuth, err := cc.details.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	encPassword, err := utils.GetEncryptedPasswordFromArtifactory(artAuth, cc.details.InsecureTls)
	if err != nil {
		return err
	}
	cc.details.Password = encPassword
	return err
}

func checkSingleAuthMethod(details *config.ServerDetails) error {
	authMethods := []bool{
		details.User != "" && details.Password != "",
		details.AccessToken != "" && details.RefreshToken == "",
		details.SshKeyPath != ""}
	if coreutils.SumTrueValues(authMethods) > 1 {
		return errorutils.CheckErrorf("Only one authentication method is allowed: Username + Password/API key, RSA Token (SSH) or Access Token")
	}
	return nil
}

type ConfigCommandConfiguration struct {
	ServerDetails *config.ServerDetails
	Interactive   bool
	EncPassword   bool
	BasicAuthOnly bool
}

func GetAllServerIds() []string {
	var serverIds []string
	configuration, err := config.GetAllServersConfigs()
	if err != nil {
		return serverIds
	}
	for _, serverConfig := range configuration {
		serverIds = append(serverIds, serverConfig.ServerId)
	}
	return serverIds
}
