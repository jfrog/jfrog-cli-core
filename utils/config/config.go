package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	cliLog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	accessAuth "github.com/jfrog/jfrog-client-go/access/auth"
	artifactoryAuth "github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/auth"
	distributionAuth "github.com/jfrog/jfrog-client-go/distribution/auth"
	lifecycleAuth "github.com/jfrog/jfrog-client-go/lifecycle/auth"
	pipelinesAuth "github.com/jfrog/jfrog-client-go/pipelines/auth"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayAuth "github.com/jfrog/jfrog-client-go/xray/auth"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	cliLog.SetDefaultLogger()
}

// This is the default server id. It is used when adding a server config without providing a server ID
const DefaultServerId = "Default-Server"

func IsServerConfExists() (bool, error) {
	conf, err := readConf()
	if err != nil {
		return false, err
	}
	return conf.Servers != nil && len(conf.Servers) > 0, nil
}

// Returns the configured server or error if the server id was not found.
// If defaultOrEmpty: return empty details if no configurations found, or default conf for empty serverId.
// Exclude refreshable tokens when working with external tools (build tools, curl, etc.) or when sending requests not via ArtifactoryHttpClient.
func GetSpecificConfig(serverId string, defaultOrEmpty bool, excludeRefreshableTokens bool) (*ServerDetails, error) {
	configs, err := GetAllServersConfigs()
	if err != nil {
		return nil, err
	}

	if defaultOrEmpty {
		if len(configs) == 0 {
			return new(ServerDetails), nil
		}
		if len(serverId) == 0 {
			details, err := GetDefaultConfiguredConf(configs)
			if excludeRefreshableTokens {
				excludeRefreshableTokensFromDetails(details)
			}
			return details, errorutils.CheckError(err)
		}
	}

	details, err := getServerConfByServerId(serverId, configs)
	if err != nil {
		return nil, err
	}
	if excludeRefreshableTokens {
		excludeRefreshableTokensFromDetails(details)
	}
	return details, nil
}

// Disables refreshable tokens if set in details.
func excludeRefreshableTokensFromDetails(details *ServerDetails) {
	if details.AccessToken != "" && details.ArtifactoryRefreshToken != "" ||
		details.AccessToken != "" && details.RefreshToken != "" {
		details.AccessToken = ""
		details.ArtifactoryRefreshToken = ""
		details.RefreshToken = ""
	}
	details.ArtifactoryTokenRefreshInterval = coreutils.TokenRefreshDisabled
}

// Returns the default server configuration or error if not found.
// Caller should perform the check error if required.
func GetDefaultConfiguredConf(configs []*ServerDetails) (*ServerDetails, error) {
	if len(configs) == 0 {
		details := new(ServerDetails)
		details.IsDefault = true
		return details, nil
	}
	for _, conf := range configs {
		if conf.IsDefault {
			return conf, nil
		}
	}
	return nil, errors.New("couldn't find default server")
}

// Returns default artifactory conf. Returns nil if default server doesn't exists.
func GetDefaultServerConf() (*ServerDetails, error) {
	configurations, err := GetAllServersConfigs()
	if err != nil {
		return nil, err
	}

	if len(configurations) == 0 {
		log.Debug("No servers were configured.")
		return nil, err
	}

	return GetDefaultConfiguredConf(configurations)
}

// Returns the configured server or error if the server id not found
func getServerConfByServerId(serverId string, configs []*ServerDetails) (*ServerDetails, error) {
	for _, conf := range configs {
		if conf.ServerId == serverId {
			return conf, nil
		}
	}
	return nil, errorutils.CheckErrorf("Server ID '%s' does not exist.", serverId)
}

func GetAndRemoveConfiguration(serverName string, configs []*ServerDetails) (*ServerDetails, []*ServerDetails) {
	for i, conf := range configs {
		if conf.ServerId == serverName {
			configs = append(configs[:i], configs[i+1:]...)
			return conf, configs
		}
	}
	return nil, configs
}

func GetAllServersConfigs() ([]*ServerDetails, error) {
	conf, err := readConf()
	if err != nil {
		return nil, err
	}
	details := conf.Servers
	if details == nil {
		return make([]*ServerDetails, 0), nil
	}
	return details, nil
}

func SaveServersConf(details []*ServerDetails) error {
	conf, err := readConf()
	if err != nil {
		return err
	}
	conf.Servers = details
	conf.Version = strconv.Itoa(coreutils.GetCliConfigVersion())
	return saveConfig(conf)
}

func saveConfig(config *Config) error {
	cloneConfig, err := config.Clone()
	if err != nil {
		return err
	}
	err = cloneConfig.encrypt()
	if err != nil {
		return err
	}

	content, err := cloneConfig.getContent()
	if err != nil {
		return err
	}

	path, err := getConfFilePath()
	if err != nil {
		return err
	}

	err = os.WriteFile(path, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func readConf() (*Config, error) {
	config := new(Config)
	content, err := getConfigFile()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		// No config file was found, returns a new empty config.
		return config, nil
	}
	content, err = convertIfNeeded(content)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &config)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	err = config.decrypt()
	return config, err
}

func getConfigFile() (content []byte, err error) {
	confFilePath, err := getConfFilePath()
	if err != nil {
		return
	}
	exists, err := fileutils.IsFileExists(confFilePath, false)
	if err != nil {
		return
	}
	if exists {
		content, err = fileutils.ReadFile(confFilePath)
		return
	}
	// Try to look for older config files
	for i := coreutils.GetCliConfigVersion() - 1; i >= 3; i-- {
		versionedConfigPath, err := getLegacyConfigFilePath(i)
		if err != nil {
			return nil, err
		}
		exists, err := fileutils.IsFileExists(versionedConfigPath, false)
		if err != nil {
			return nil, err
		}
		if exists {
			// If an old config file was found returns its content or an error.
			content, err = fileutils.ReadFile(versionedConfigPath)
			return content, err
		}
	}

	return content, nil
}

func (config *Config) Clone() (*Config, error) {
	bytes, err := json.Marshal(config)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	clone := &Config{}
	if err = json.Unmarshal(bytes, clone); err != nil {
		return nil, errorutils.CheckError(err)
	}
	return clone, nil
}

func (config *Config) getContent() ([]byte, error) {
	b, err := json.Marshal(&config)
	if err != nil {
		return []byte{}, errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, b, "", "  ")
	if err != nil {
		return []byte{}, errorutils.CheckError(err)
	}
	return content.Bytes(), nil
}

// Move SSL certificates from the old location in security dir to certs dir.
func convertCertsDir() error {
	securityDir, err := coreutils.GetJfrogSecurityDir()
	if err != nil {
		return err
	}
	exists, err := fileutils.IsDirExists(securityDir, false)
	// Security dir doesn't exist, no conversion needed.
	if err != nil || !exists {
		return err
	}

	certsDir, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return err
	}
	exists, err = fileutils.IsDirExists(certsDir, false)
	// Certs dir already exists, no conversion needed.
	if err != nil || exists {
		return err
	}

	// Move certs to the new location.
	files, err := os.ReadDir(securityDir)
	if err != nil {
		return errorutils.CheckError(err)
	}

	log.Debug("Migrating SSL certificates to the new location at: " + certsDir)
	for _, f := range files {
		// Skip directories and the security configuration file
		if !f.IsDir() && f.Name() != coreutils.JfrogSecurityConfFile {
			err = fileutils.CreateDirIfNotExist(certsDir)
			if err != nil {
				return err
			}
			err = os.Rename(filepath.Join(securityDir, f.Name()), filepath.Join(certsDir, f.Name()))
			if err != nil {
				return errorutils.CheckError(err)
			}
		}
	}
	return nil
}

// The configuration schema can change between versions, therefore we need to convert old versions to the new schema.
func convertIfNeeded(content []byte) ([]byte, error) {
	version, err := getVersion(content)
	if err != nil {
		return nil, err
	}

	// Switch contains FALLTHROUGH to convert from a certain version to the latest.
	switch version {
	case strconv.Itoa(coreutils.GetCliConfigVersion()):
		return content, nil
	case "0":
		content, err = convertConfigV0toV1(content)
		if err != nil {
			return nil, err
		}
		fallthrough
	case "1":
		err = createHomeDirBackup()
		if err != nil {
			return nil, err
		}
		err = convertCertsDir()
		if err != nil {
			return nil, err
		}
		fallthrough
	case "2":
		content, err = convertConfigV2toV3(content)
		if err != nil {
			return nil, err
		}
		fallthrough
	case "3", "4":
		content, err = convertConfigV4toV5(content)
		if err != nil {
			return nil, err
		}
		fallthrough
	case "5":
		content, err = convertConfigV5toV6(content)
	}
	if err != nil {
		return nil, err
	}

	// Save config after all conversions (also updates version).
	result := new(Config)
	err = json.Unmarshal(content, &result)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	result.Version = strconv.Itoa(coreutils.GetCliConfigVersion())
	err = saveConfig(result)
	if err != nil {
		return nil, err
	}
	content, err = json.Marshal(&result)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return content, err
}

// Creating a homedir backup prior to converting.
func createHomeDirBackup() error {
	homeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return err
	}
	backupDir, err := coreutils.GetJfrogBackupDir()
	if err != nil {
		return err
	}

	// Copy homedir contents to backup dir, excluding redundant dirs and the backup dir itself.
	backupName := ".jfrog-" + strconv.FormatInt(time.Now().Unix(), 10)
	curBackupPath := filepath.Join(backupDir, backupName)
	log.Debug("Creating a homedir backup at: " + curBackupPath)
	exclude := []string{coreutils.JfrogBackupDirName, coreutils.JfrogDependenciesDirName, coreutils.JfrogLocksDirName, coreutils.JfrogLogsDirName}
	return fileutils.CopyDir(homeDir, curBackupPath, true, exclude)
}

// Version key doesn't exist in version 0
// Version key is "Version" in version 1
// Version key is "version" in version 2 and above
func getVersion(content []byte) (value string, err error) {
	value, err = jsonparser.GetString(bytes.ToLower(content), "version")
	if err != nil && err.Error() == "Key path not found" {
		return "0", nil
	}
	return value, errorutils.CheckError(err)
}

func convertConfigV0toV1(content []byte) ([]byte, error) {
	result := new(ConfigV4)
	configV0 := new(ConfigV0)
	err := json.Unmarshal(content, &configV0)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	result = configV0.Convert()
	result.Version = "1"
	content, err = json.Marshal(&result)
	return content, errorutils.CheckError(err)
}

func convertConfigV2toV3(content []byte) ([]byte, error) {
	config := new(ConfigV4)
	err := json.Unmarshal(content, &config)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	for _, rtConfig := range config.Artifactory {
		rtConfig.User = strings.ToLower(rtConfig.User)
	}
	content, err = json.Marshal(&config)
	return content, errorutils.CheckError(err)
}

func convertConfigV4toV5(content []byte) ([]byte, error) {
	config := new(ConfigV4)
	err := json.Unmarshal(content, &config)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}

	result := config.Convert()
	content, err = json.Marshal(&result)
	return content, errorutils.CheckError(err)
}

func convertConfigV5toV6(content []byte) ([]byte, error) {
	config := new(ConfigV5)
	err := json.Unmarshal(content, &config)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}

	result := config.Convert()
	content, err = json.Marshal(&result)
	return content, errorutils.CheckError(err)
}

func GetJfrogDependenciesPath() (string, error) {
	dependenciesDir := os.Getenv(coreutils.DependenciesDir)
	if dependenciesDir != "" {
		return utils.AddTrailingSlashIfNeeded(dependenciesDir), nil
	}
	jfrogHome, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(jfrogHome, coreutils.JfrogDependenciesDirName), nil
}

func getConfFilePath() (string, error) {
	confPath, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(confPath, 0777)
	if err != nil {
		return "", err
	}

	versionString := ".v" + strconv.Itoa(coreutils.GetCliConfigVersion())
	confPath = filepath.Join(confPath, coreutils.JfrogConfigFile+versionString)
	return confPath, nil
}

func getLegacyConfigFilePath(version int) (string, error) {
	confPath, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	confPath = filepath.Join(confPath, coreutils.JfrogConfigFile)
	// Before version 4 all the config files were saved with the same name.
	if version < 4 {
		return confPath, nil
	}
	return confPath + ".v" + strconv.Itoa(version), nil

}

// Config represents the CLI latest config version.
type Config struct {
	ConfigV6
}

type ConfigV6 struct {
	ConfigV5
}

type ConfigV5 struct {
	Servers []*ServerDetails `json:"servers"`
	Version string           `json:"version,omitempty"`
	Enc     bool             `json:"enc,omitempty"`
}

// This struct is suitable for versions 1, 2, 3 and 4.
type ConfigV4 struct {
	Artifactory    []*ServerDetails       `json:"artifactory"`
	MissionControl *MissionControlDetails `json:"missionControl,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Enc            bool                   `json:"enc,omitempty"`
}

func (o *ConfigV5) Convert() *ConfigV6 {
	config := new(ConfigV6)
	config.Servers = o.Servers
	for _, server := range config.Servers {
		server.ArtifactoryRefreshToken = server.RefreshToken
		server.RefreshToken = ""
	}
	return config
}

func (o *ConfigV4) Convert() *ConfigV5 {
	config := new(ConfigV5)
	config.Servers = o.Artifactory
	for _, server := range config.Servers {
		server.ArtifactoryUrl = server.Url
		server.Url = ""
		if server.IsDefault && o.MissionControl != nil {
			server.MissionControlUrl = o.MissionControl.Url
		}
	}
	return config
}

// This struct was created before the version property was added to the config.
type ConfigV0 struct {
	Artifactory    *ServerDetails         `json:"artifactory,omitempty"`
	MissionControl *MissionControlDetails `json:"MissionControl,omitempty"`
}

func (o *ConfigV0) Convert() *ConfigV4 {
	config := new(ConfigV4)
	config.MissionControl = o.MissionControl
	if o.Artifactory != nil {
		o.Artifactory.IsDefault = true
		o.Artifactory.ServerId = DefaultServerId
		config.Artifactory = []*ServerDetails{o.Artifactory}
	}
	return config
}

type ServerDetails struct {
	Url                             string `json:"url,omitempty"`
	SshUrl                          string `json:"-"`
	ArtifactoryUrl                  string `json:"artifactoryUrl,omitempty"`
	DistributionUrl                 string `json:"distributionUrl,omitempty"`
	XrayUrl                         string `json:"xrayUrl,omitempty"`
	MissionControlUrl               string `json:"missionControlUrl,omitempty"`
	PipelinesUrl                    string `json:"pipelinesUrl,omitempty"`
	AccessUrl                       string `json:"accessUrl,omitempty"`
	LifecycleUrl                    string `json:"-"`
	User                            string `json:"user,omitempty"`
	Password                        string `json:"password,omitempty"`
	SshKeyPath                      string `json:"sshKeyPath,omitempty"`
	SshPassphrase                   string `json:"sshPassphrase,omitempty"`
	AccessToken                     string `json:"accessToken,omitempty"`
	RefreshToken                    string `json:"refreshToken,omitempty"`
	ArtifactoryRefreshToken         string `json:"artifactoryRefreshToken,omitempty"`
	ArtifactoryTokenRefreshInterval int    `json:"tokenRefreshInterval,omitempty"`
	ClientCertPath                  string `json:"clientCertPath,omitempty"`
	ClientCertKeyPath               string `json:"clientCertKeyPath,omitempty"`
	ServerId                        string `json:"serverId,omitempty"`
	IsDefault                       bool   `json:"isDefault,omitempty"`
	InsecureTls                     bool   `json:"-"`
	WebLogin                        bool   `json:"webLogin,omitempty"`
}

// Deprecated
type MissionControlDetails struct {
	Url         string `json:"url,omitempty"`
	AccessToken string `json:"accessToken,omitempty"`
}

func (serverDetails *ServerDetails) IsEmpty() bool {
	return len(serverDetails.ServerId) == 0 && serverDetails.Url == ""
}

func (serverDetails *ServerDetails) SetUser(username string) {
	serverDetails.User = username
}

func (serverDetails *ServerDetails) SetPassword(password string) {
	serverDetails.Password = password
}

func (serverDetails *ServerDetails) SetAccessToken(accessToken string) {
	serverDetails.AccessToken = accessToken
}

func (serverDetails *ServerDetails) SetArtifactoryRefreshToken(refreshToken string) {
	serverDetails.ArtifactoryRefreshToken = refreshToken
}

func (serverDetails *ServerDetails) SetRefreshToken(refreshToken string) {
	serverDetails.RefreshToken = refreshToken
}

func (serverDetails *ServerDetails) SetSshPassphrase(sshPassphrase string) {
	serverDetails.SshPassphrase = sshPassphrase
}

func (serverDetails *ServerDetails) SetClientCertPath(certificatePath string) {
	serverDetails.ClientCertPath = certificatePath
}

func (serverDetails *ServerDetails) SetClientCertKeyPath(certificatePath string) {
	serverDetails.ClientCertKeyPath = certificatePath
}

func (serverDetails *ServerDetails) GetUrl() string {
	return serverDetails.Url
}

func (serverDetails *ServerDetails) GetArtifactoryUrl() string {
	return serverDetails.ArtifactoryUrl
}

func (serverDetails *ServerDetails) GetDistributionUrl() string {
	return serverDetails.DistributionUrl
}

func (serverDetails *ServerDetails) GetXrayUrl() string {
	return serverDetails.XrayUrl
}

func (serverDetails *ServerDetails) GetMissionControlUrl() string {
	return serverDetails.MissionControlUrl
}

func (serverDetails *ServerDetails) GetPipelinesUrl() string {
	return serverDetails.PipelinesUrl
}

func (serverDetails *ServerDetails) GetAccessUrl() string {
	return serverDetails.AccessUrl
}

func (serverDetails *ServerDetails) GetLifecycleUrl() string {
	return serverDetails.LifecycleUrl
}

func (serverDetails *ServerDetails) GetUser() string {
	return serverDetails.User
}

func (serverDetails *ServerDetails) GetPassword() string {
	return serverDetails.Password
}

func (serverDetails *ServerDetails) GetAccessToken() string {
	return serverDetails.AccessToken
}

func (serverDetails *ServerDetails) GetRefreshToken() string {
	return serverDetails.RefreshToken
}

func (serverDetails *ServerDetails) GetClientCertPath() string {
	return serverDetails.ClientCertPath
}

func (serverDetails *ServerDetails) GetClientCertKeyPath() string {
	return serverDetails.ClientCertKeyPath
}

func (serverDetails *ServerDetails) CreateArtAuthConfig() (auth.ServiceDetails, error) {
	artAuth := artifactoryAuth.NewArtifactoryDetails()
	artAuth.SetUrl(serverDetails.ArtifactoryUrl)
	return serverDetails.createAuthConfig(artAuth)
}

func (serverDetails *ServerDetails) CreateDistAuthConfig() (auth.ServiceDetails, error) {
	artAuth := distributionAuth.NewDistributionDetails()
	artAuth.SetUrl(serverDetails.DistributionUrl)
	return serverDetails.createAuthConfig(artAuth)
}

func (serverDetails *ServerDetails) CreateXrayAuthConfig() (auth.ServiceDetails, error) {
	artAuth := xrayAuth.NewXrayDetails()
	artAuth.SetUrl(serverDetails.XrayUrl)
	return serverDetails.createAuthConfig(artAuth)
}

func (serverDetails *ServerDetails) CreatePipelinesAuthConfig() (auth.ServiceDetails, error) {
	pAuth := pipelinesAuth.NewPipelinesDetails()
	pAuth.SetUrl(serverDetails.PipelinesUrl)
	return serverDetails.createAuthConfig(pAuth)
}

func (serverDetails *ServerDetails) CreateAccessAuthConfig() (auth.ServiceDetails, error) {
	pAuth := accessAuth.NewAccessDetails()
	pAuth.SetUrl(utils.AddTrailingSlashIfNeeded(serverDetails.Url) + "access/")
	return serverDetails.createAuthConfig(pAuth)
}

func (serverDetails *ServerDetails) CreateLifecycleAuthConfig() (auth.ServiceDetails, error) {
	lcAuth := lifecycleAuth.NewLifecycleDetails()
	lcAuth.SetUrl(serverDetails.LifecycleUrl)
	return serverDetails.createAuthConfig(lcAuth)
}

func (serverDetails *ServerDetails) createAuthConfig(details auth.ServiceDetails) (auth.ServiceDetails, error) {
	details.SetSshUrl(serverDetails.SshUrl)
	details.SetAccessToken(serverDetails.AccessToken)
	// If refresh token is not empty, set a refresh handler and skip other credentials.
	// First we check access's token, if empty we check artifactory's token.
	switch {
	case serverDetails.RefreshToken != "":
		// Save serverId for refreshing if needed. If empty serverId is saved, default will be used.
		tokenRefreshServerId = serverDetails.ServerId
		details.AppendPreRequestFunction(AccessTokenRefreshPreRequestInterceptor)
	case serverDetails.ArtifactoryRefreshToken != "":
		// Save serverId for refreshing if needed. If empty serverId is saved, default will be used.
		tokenRefreshServerId = serverDetails.ServerId
		details.AppendPreRequestFunction(ArtifactoryTokenRefreshPreRequestInterceptor)
	default:
		details.SetUser(serverDetails.User)
		details.SetPassword(serverDetails.Password)
	}
	details.SetClientCertPath(serverDetails.ClientCertPath)
	details.SetClientCertKeyPath(serverDetails.ClientCertKeyPath)
	details.SetSshKeyPath(serverDetails.SshKeyPath)
	details.SetSshPassphrase(serverDetails.SshPassphrase)
	return details, nil
}

func (missionControlDetails *MissionControlDetails) GetAccessToken() string {
	return missionControlDetails.AccessToken
}

func (missionControlDetails *MissionControlDetails) SetAccessToken(accessToken string) {
	missionControlDetails.AccessToken = accessToken
}
