package commands

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type ConfigAction string

const (
	AddOrEdit ConfigAction = "AddOrEdit"
	Delete    ConfigAction = "Delete"
	Use       ConfigAction = "Use"
	Clear     ConfigAction = "Clear"
)

type AuthenticationMethod string

const (
	AccessToken AuthenticationMethod = "Access Token"
	BasicAuth   AuthenticationMethod = "Username and Password / API Key"
	MTLS        AuthenticationMethod = "Mutual TLS"
	WebLogin    AuthenticationMethod = "Web Login"
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
	// Preselected web login authentication method, supported on an interactive command only.
	useWebLogin bool
	// Forcibly make the configured server default.
	makeDefault bool
	// For unit tests
	disablePrompts bool
	cmdType        ConfigAction
}

func NewConfigCommand(cmdType ConfigAction, serverId string) *ConfigCommand {
	return &ConfigCommand{cmdType: cmdType, serverId: serverId}
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

func (cc *ConfigCommand) SetUseWebLogin(useWebLogin bool) *ConfigCommand {
	cc.useWebLogin = useWebLogin
	return cc
}

func (cc *ConfigCommand) SetMakeDefault(makeDefault bool) *ConfigCommand {
	cc.makeDefault = makeDefault
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

func (cc *ConfigCommand) Run() (err error) {
	log.Debug("Locking config file to run config " + cc.cmdType + " command.")
	mutex.Lock()
	defer func() {
		mutex.Unlock()
		log.Debug("Config " + cc.cmdType + " command completed successfully. config file is released.")
	}()

	lockDirPath, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return
	}
	unlockFunc, err := lock.CreateLock(lockDirPath)
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		e := unlockFunc()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return
	}

	switch cc.cmdType {
	case AddOrEdit:
		err = cc.config()
	case Delete:
		err = cc.delete()
	case Use:
		err = cc.use()
	case Clear:
		err = cc.clear()
	default:
		err = fmt.Errorf("Not supported config command type: " + string(cc.cmdType))
	}
	return
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

func (cc *ConfigCommand) config() error {
	configurations, err := cc.prepareConfigurationData()
	if err != nil {
		return err
	}
	if cc.interactive {
		err = cc.getConfigurationFromUser()
	} else {
		err = cc.getConfigurationNonInteractively()
	}
	if err != nil {
		return err
	}
	cc.addTrailingSlashes()
	cc.lowerUsername()
	cc.setDefaultIfNeeded(configurations)
	if err = checkSingleAuthMethod(cc.details); err != nil {
		return err
	}
	if err = cc.assertUrlsSafe(); err != nil {
		return err
	}
	if err = cc.encPasswordIfNeeded(); err != nil {
		return err
	}
	cc.configRefreshableTokenIfPossible()
	return config.SaveServersConf(configurations)
}

func (cc *ConfigCommand) getConfigurationNonInteractively() error {
	if cc.details.Url != "" {
		if fileutils.IsSshUrl(cc.details.Url) {
			coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url)
		} else {
			cc.details.Url = clientUtils.AddTrailingSlashIfNeeded(cc.details.Url)
			// Derive JFrog services URLs from platform URL
			coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url+"artifactory/")
			coreutils.SetIfEmpty(&cc.details.DistributionUrl, cc.details.Url+"distribution/")
			coreutils.SetIfEmpty(&cc.details.XrayUrl, cc.details.Url+"xray/")
			coreutils.SetIfEmpty(&cc.details.MissionControlUrl, cc.details.Url+"mc/")
			coreutils.SetIfEmpty(&cc.details.PipelinesUrl, cc.details.Url+"pipelines/")
		}
	}

	if cc.details.AccessToken != "" && cc.details.User == "" {
		if err := cc.validateTokenIsNotApiKey(); err != nil {
			return err
		}
		cc.tryExtractingUsernameFromAccessToken()
	}
	return nil
}

func (cc *ConfigCommand) addTrailingSlashes() {
	cc.details.ArtifactoryUrl = clientUtils.AddTrailingSlashIfNeeded(cc.details.ArtifactoryUrl)
	cc.details.DistributionUrl = clientUtils.AddTrailingSlashIfNeeded(cc.details.DistributionUrl)
	cc.details.XrayUrl = clientUtils.AddTrailingSlashIfNeeded(cc.details.XrayUrl)
	cc.details.MissionControlUrl = clientUtils.AddTrailingSlashIfNeeded(cc.details.MissionControlUrl)
	cc.details.PipelinesUrl = clientUtils.AddTrailingSlashIfNeeded(cc.details.PipelinesUrl)
}

// Artifactory expects the username to be lower-cased. In case it is not,
// Artifactory will silently save it lower-cased, but the token creation
// REST API will fail with a non-lower-cased username.
func (cc *ConfigCommand) lowerUsername() {
	cc.details.User = strings.ToLower(cc.details.User)
}

func (cc *ConfigCommand) setDefaultIfNeeded(configurations []*config.ServerDetails) {
	if len(configurations) == 1 {
		cc.details.IsDefault = true
		return
	}
	if cc.makeDefault {
		for i := range configurations {
			configurations[i].IsDefault = false
		}
		cc.details.IsDefault = true
	}
}

func (cc *ConfigCommand) encPasswordIfNeeded() error {
	if cc.encPassword && cc.details.ArtifactoryUrl != "" {
		err := cc.encryptPassword()
		if err != nil {
			return errorutils.CheckErrorf("The following error was received while trying to encrypt your password: %s ", err)
		}
	}
	return nil
}

func (cc *ConfigCommand) configRefreshableTokenIfPossible() {
	if cc.useBasicAuthOnly {
		return
	}
	// If username and password weren't provided, then the artifactoryToken refresh mechanism isn't set.
	if cc.details.User == "" || cc.details.Password == "" {
		return
	}
	// Set the default interval for the refreshable tokens to be initialized in the next CLI run.
	cc.details.ArtifactoryTokenRefreshInterval = coreutils.TokenRefreshDefaultInterval
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
		ioutils.ScanFromConsole("Choose a server ID", &cc.serverId, cc.defaultDetails.ServerId)
	}
	cc.details.ServerId = cc.resolveServerId()

	// Remove and get the server details from the configurations list
	tempConfiguration, configurations := config.GetAndRemoveConfiguration(cc.details.ServerId, configurations)

	// Change default server details if the server existed in the configurations list
	if tempConfiguration != nil {
		cc.defaultDetails = tempConfiguration
		cc.details.IsDefault = tempConfiguration.IsDefault
	}

	// Append the configuration to the configurations list
	configurations = append(configurations, cc.details)
	return configurations, err
}

// Returning the first non-empty value:
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

func (cc *ConfigCommand) getConfigurationFromUser() (err error) {
	if cc.details.Url == "" {
		ioutils.ScanFromConsole("JFrog Platform URL", &cc.details.Url, cc.defaultDetails.Url)
	}

	var disallowUsingSavedPassword bool
	if fileutils.IsSshUrl(cc.details.Url) {
		coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url)
	} else {
		disallowUsingSavedPassword = cc.fillUrlsPrePrompting()
	}

	if fileutils.IsSshUrl(cc.details.ArtifactoryUrl) {
		if err = getSshKeyPath(cc.details); err != nil {
			return
		}
	} else if !cc.disablePrompts {
		if err = cc.promptUrls(&disallowUsingSavedPassword); err != nil {
			return
		}
		var clientCertChecked bool
		if cc.details.Password == "" && cc.details.AccessToken == "" {
			clientCertChecked, err = cc.promptForCredentials(disallowUsingSavedPassword)
			if err != nil {
				return err
			}
		}
		if !clientCertChecked {
			cc.checkClientCertForReverseProxy()
		}
	}
	return
}

func (cc *ConfigCommand) fillUrlsPrePrompting() (disallowUsingSavedPassword bool) {
	cc.details.Url = clientUtils.AddTrailingSlashIfNeeded(cc.details.Url)
	disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.DistributionUrl, cc.details.Url+"distribution/") || disallowUsingSavedPassword
	disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.ArtifactoryUrl, cc.details.Url+"artifactory/") || disallowUsingSavedPassword
	disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.XrayUrl, cc.details.Url+"xray/") || disallowUsingSavedPassword
	disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.MissionControlUrl, cc.details.Url+"mc/") || disallowUsingSavedPassword
	disallowUsingSavedPassword = coreutils.SetIfEmpty(&cc.details.PipelinesUrl, cc.details.Url+"pipelines/") || disallowUsingSavedPassword
	return
}

func (cc *ConfigCommand) checkCertificateForMTLS() {
	if cc.details.ClientCertPath != "" && cc.details.ClientCertKeyPath != "" {
		return
	}
	cc.readClientCertInfoFromConsole()
}

func (cc *ConfigCommand) promptAuthMethods() (selectedMethod AuthenticationMethod, err error) {
	if cc.useWebLogin {
		return WebLogin, nil
	}

	var selected string
	authMethods := []AuthenticationMethod{
		BasicAuth,
		AccessToken,
		MTLS,
		// WebLogin, // TODO uncomment when Artifactory 7.63.1 is released to self-hosted
	}
	var selectableItems []ioutils.PromptItem
	for _, curMethod := range authMethods {
		selectableItems = append(selectableItems, ioutils.PromptItem{Option: string(curMethod), TargetValue: &selected})
	}
	err = ioutils.SelectString(selectableItems, "Select one of the following authentication methods:", false, func(item ioutils.PromptItem) {
		*item.TargetValue = item.Option
		selectedMethod = AuthenticationMethod(*item.TargetValue)
	})
	return
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

func (cc *ConfigCommand) promptForCredentials(disallowUsingSavedPassword bool) (clientCertChecked bool, err error) {
	var authMethod AuthenticationMethod
	authMethod, err = cc.promptAuthMethods()
	if err != nil {
		return
	}
	switch authMethod {
	case BasicAuth:
		return false, ioutils.ReadCredentialsFromConsole(cc.details, cc.defaultDetails, disallowUsingSavedPassword)
	case AccessToken:
		return false, cc.promptForAccessToken()
	case MTLS:
		cc.checkCertificateForMTLS()
		log.Warn("Please notice that authentication using client certificates (mTLS) is not supported by commands which integrate with package managers.")
		return true, nil
	case WebLogin:
		// Web login sends requests, so certificates must be obtained first if they are required.
		cc.checkClientCertForReverseProxy()
		return true, cc.handleWebLogin()
	default:
		return false, errorutils.CheckErrorf("unexpected authentication method")
	}
}

func (cc *ConfigCommand) promptForAccessToken() error {
	if err := readAccessTokenFromConsole(cc.details); err != nil {
		return err
	}
	if err := cc.validateTokenIsNotApiKey(); err != nil {
		return err
	}
	if cc.details.User == "" {
		cc.tryExtractingUsernameFromAccessToken()
		if cc.details.User == "" {
			ioutils.ScanFromConsole("JFrog username (optional)", &cc.details.User, "")
		}
	}
	return nil
}

// Some package managers support basic authentication only. To support them, we try to extract the username from the access token.
// This is not feasible with reference token.
func (cc *ConfigCommand) tryExtractingUsernameFromAccessToken() {
	cc.details.User = auth.ExtractUsernameFromAccessToken(cc.details.AccessToken)
}

func (cc *ConfigCommand) readClientCertInfoFromConsole() {
	if cc.details.ClientCertPath == "" {
		ioutils.ScanFromConsole("Client certificate file path", &cc.details.ClientCertPath, cc.defaultDetails.ClientCertPath)
	}
	if cc.details.ClientCertKeyPath == "" {
		ioutils.ScanFromConsole("Client certificate key path", &cc.details.ClientCertKeyPath, cc.defaultDetails.ClientCertKeyPath)
	}
}

func (cc *ConfigCommand) checkClientCertForReverseProxy() {
	if cc.details.ClientCertPath != "" && cc.details.ClientCertKeyPath != "" {
		return
	}
	if coreutils.AskYesNo("Is the Artifactory reverse proxy configured to accept a client certificate?", false) {
		cc.readClientCertInfoFromConsole()
	}
}

func readAccessTokenFromConsole(details *config.ServerDetails) error {
	token, err := ioutils.ScanPasswordFromConsole("JFrog access token:")
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
	details.SshKeyPath = clientUtils.ReplaceTildeWithUserHome(details.SshKeyPath)
	exists, err := fileutils.IsFileExists(details.SshKeyPath, false)
	if err != nil {
		return err
	}

	messageSuffix := ": "
	if exists {
		sshKeyBytes, err := os.ReadFile(details.SshKeyPath)
		if err != nil {
			return err
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

func Import(configTokenString string) error {
	serverDetails, err := config.Import(configTokenString)
	if err != nil {
		return err
	}
	log.Info("Importing server ID", "'"+serverDetails.ServerId+"'")
	configCommand := &ConfigCommand{
		details:  serverDetails,
		serverId: serverDetails.ServerId,
	}
	return configCommand.config()
}

func Export(serverName string) error {
	serverDetails, err := config.GetSpecificConfig(serverName, true, false)
	if err != nil {
		return err
	}
	if serverDetails.ServerId == "" {
		return errorutils.CheckErrorf("cannot export config, because it is empty. Run 'jf c add' and then export again")
	}
	configTokenString, err := config.Export(serverDetails)
	if err != nil {
		return err
	}
	log.Output(configTokenString)
	return nil
}

func moveDefaultConfigToSliceEnd(configuration []*config.ServerDetails) []*config.ServerDetails {
	lastIndex := len(configuration) - 1
	// If configuration list has more than one config and the last one is not default, switch the last default config with the last one
	if len(configuration) > 1 && !configuration[lastIndex].IsDefault {
		for i, server := range configuration {
			if server.IsDefault {
				configuration[i] = configuration[lastIndex]
				configuration[lastIndex] = server
				break
			}
		}
	}
	return configuration
}

func printConfigs(configuration []*config.ServerDetails) {
	// Make default config to be the last config, so it will be easy to see on the terminal
	configuration = moveDefaultConfigToSliceEnd(configuration)

	for _, details := range configuration {
		isDefault := details.IsDefault
		logIfNotEmpty(details.ServerId, "Server ID:\t\t\t", false, isDefault)
		logIfNotEmpty(details.Url, "JFrog Platform URL:\t\t", false, isDefault)
		logIfNotEmpty(details.ArtifactoryUrl, "Artifactory URL:\t\t", false, isDefault)
		logIfNotEmpty(details.DistributionUrl, "Distribution URL:\t\t", false, isDefault)
		logIfNotEmpty(details.XrayUrl, "Xray URL:\t\t\t", false, isDefault)
		logIfNotEmpty(details.MissionControlUrl, "Mission Control URL:\t\t", false, isDefault)
		logIfNotEmpty(details.PipelinesUrl, "Pipelines URL:\t\t\t", false, isDefault)
		logIfNotEmpty(details.User, "User:\t\t\t\t", false, isDefault)
		logIfNotEmpty(details.Password, "Password:\t\t\t", true, isDefault)
		logAccessTokenIfNotEmpty(details.AccessToken, isDefault)
		logIfNotEmpty(details.RefreshToken, "Refresh token:\t\t\t", true, isDefault)
		logIfNotEmpty(details.SshKeyPath, "SSH key file path:\t\t", false, isDefault)
		logIfNotEmpty(details.SshPassphrase, "SSH passphrase:\t\t\t", true, isDefault)
		logIfNotEmpty(details.ClientCertPath, "Client certificate file path:\t", false, isDefault)
		logIfNotEmpty(details.ClientCertKeyPath, "Client certificate key path:\t", false, isDefault)
		logIfNotEmpty(strconv.FormatBool(details.IsDefault), "Default:\t\t\t", false, isDefault)
		log.Output()
	}
}

func logIfNotEmpty(value, prefix string, mask, isDefault bool) {
	if value != "" {
		if mask {
			value = "***"
		}
		fullString := prefix + value
		if isDefault {
			fullString = coreutils.PrintBoldTitle(fullString)
		}
		log.Output(fullString)
	}
}

func logAccessTokenIfNotEmpty(token string, isDefault bool) {
	if token == "" {
		return
	}
	tokenString := "***"
	// Extract the token's subject only if it is JWT
	if strings.Count(token, ".") == 2 {
		subject, err := auth.ExtractSubjectFromAccessToken(token)
		if err != nil {
			log.Error(err)
		} else {
			tokenString += fmt.Sprintf(" (Subject: '%s')", subject)
		}
	}

	logIfNotEmpty(tokenString, "Access token:\t\t\t", false, isDefault)
}

func (cc *ConfigCommand) delete() error {
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	var isDefault, isFoundName bool
	for i, serverDetails := range configurations {
		if serverDetails.ServerId == cc.serverId {
			isDefault = serverDetails.IsDefault
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
	log.Info("\"" + cc.serverId + "\" configuration could not be found.\n")
	return nil
}

// Set the default configuration
func (cc *ConfigCommand) use() error {
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	var serverFound *config.ServerDetails
	newDefaultServer := true
	for _, serverDetails := range configurations {
		if serverDetails.ServerId == cc.serverId {
			serverFound = serverDetails
			if serverDetails.IsDefault {
				newDefaultServer = false
				break
			}
			serverDetails.IsDefault = true
		} else {
			serverDetails.IsDefault = false
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
		usingServerLog := fmt.Sprintf("Using server ID '%s'", serverFound.ServerId)
		if serverFound.Url != "" {
			usingServerLog += fmt.Sprintf(" (%s)", serverFound.Url)
		} else if serverFound.ArtifactoryUrl != "" {
			usingServerLog += fmt.Sprintf(" (%s)", serverFound.ArtifactoryUrl)
		}
		log.Info(usingServerLog)
		return nil
	}
	return errorutils.CheckErrorf("Could not find a server with ID '%s'.", cc.serverId)
}

func (cc *ConfigCommand) clear() error {
	if cc.interactive {
		confirmed := coreutils.AskYesNo("Are you sure you want to delete all the configurations?", false)
		if !confirmed {
			return nil
		}
	}
	return config.SaveServersConf(make([]*config.ServerDetails, 0))
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

// Assert all services URLs are safe
func (cc *ConfigCommand) assertUrlsSafe() error {
	for _, curUrl := range []string{cc.details.Url, cc.details.AccessUrl, cc.details.ArtifactoryUrl,
		cc.details.DistributionUrl, cc.details.MissionControlUrl, cc.details.PipelinesUrl, cc.details.XrayUrl} {
		if isUrlSafe(curUrl) {
			continue
		}
		if cc.interactive {
			if cc.disablePrompts || !coreutils.AskYesNo("Your JFrog URL uses an insecure HTTP connection, instead of HTTPS. Are you sure you want to continue?", false) {
				return errorutils.CheckErrorf("config was aborted due to an insecure HTTP connection")
			}
		} else {
			log.Warn("Your configured JFrog URL uses an insecure HTTP connection. Please consider using SSL (HTTPS instead of HTTP).")
		}
		return nil
	}
	return nil
}

func (cc *ConfigCommand) validateTokenIsNotApiKey() error {
	if httpclient.IsApiKey(cc.details.AccessToken) {
		return errors.New("the provided Access Token is an API key and should be used as a password in username/password authentication")
	}
	return nil
}

func (cc *ConfigCommand) handleWebLogin() error {
	token, err := utils.DoWebLogin(cc.details)
	if err != nil {
		return err
	}
	cc.details.AccessToken = token.AccessToken
	cc.details.RefreshToken = token.RefreshToken
	cc.details.WebLogin = true
	cc.tryExtractingUsernameFromAccessToken()
	return nil
}

// Return true if a URL is safe. URL is considered not safe if the following conditions are met:
// 1. The URL uses an http:// scheme
// 2. The URL leads to a URL outside the local machine
func isUrlSafe(urlToCheck string) bool {
	parsedUrl, err := url.Parse(urlToCheck)
	if err != nil {
		// If the URL cannot be parsed, we treat it as safe.
		return true
	}

	if parsedUrl.Scheme != "http" {
		return true
	}

	hostName := parsedUrl.Hostname()
	if hostName == "127.0.0.1" || hostName == "localhost" {
		return true
	}

	return false
}

func checkSingleAuthMethod(details *config.ServerDetails) error {
	authMethods := []bool{
		details.User != "" && details.Password != "",
		details.AccessToken != "" && details.ArtifactoryRefreshToken == "",
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
