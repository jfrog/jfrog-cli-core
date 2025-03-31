package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	configtests "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetDefaultLogger()
}

func TestCovertConfigV0ToV1(t *testing.T) {
	configV0 := `
		{
		  "artifactory": {
			  "url": "http://localhost:8080/artifactory/",
			  "user": "user",
			  "password": "password"
		  }
		}
	`
	content, err := convertConfigV0toV1([]byte(configV0))
	assert.NoError(t, err)
	configV1 := new(ConfigV4)
	assert.NoError(t, json.Unmarshal(content, &configV1))
	assertionV4Helper(t, configV1, 1, false)
}

func TestConvertConfigV0ToLatest(t *testing.T) {
	configV0 := `
		{
		  "artifactory": {
			  "url": "http://localhost:8080/artifactory/",
			  "user": "user",
			  "password": "password"
		  },
		  "missioncontrol": {
			  "url": "http://localhost:8080/mc/"
		  }
		}
	`

	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(configV0))
	assert.NoError(t, err)
	configV6 := new(ConfigV6)
	assert.NoError(t, json.Unmarshal(content, &configV6))
	assertionHelper(t, configV6, 0)
	assertCertsMigrationAndBackupCreation(t)
}

func TestConvertConfigV1ToLatest(t *testing.T) {
	// The Artifactory username is uppercase intentionally,
	// to test the lowercase conversion to version 3.
	config := `
		{
		  "artifactory": [
			{
			  "url": "http://localhost:8080/artifactory/",
			  "user": "USER",
			  "password": "password",
			  "serverId": "` + DefaultServerId + `",
			  "isDefault": true
			}
		  ],
		  "missioncontrol": {
			"url": "http://localhost:8080/mc/"
		  },
		  "Version": "1"
		}
	`

	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(config))
	assert.NoError(t, err)
	configV6 := new(ConfigV6)
	assert.NoError(t, json.Unmarshal(content, &configV6))
	assertionHelper(t, configV6, 1)

	assert.Equal(t, "user", configV6.Servers[0].User, "The config conversion to version 3 is supposed to save the username as lowercase")

	assertCertsMigrationAndBackupCreation(t)
}

func assertCertsMigrationAndBackupCreation(t *testing.T) {
	assertCertsMigration(t)
	backupDir, err := coreutils.GetJfrogBackupDir()
	assert.NoError(t, err)
	assert.DirExists(t, backupDir)
}

func TestConvertConfigV4ToLatest(t *testing.T) {
	configV4 := `
		{
		  "artifactory": [
			  {
			  	"url": "http://localhost:8080/artifactory/",
			 	"user": "user",
				"password": "password",
				"serverId": "` + DefaultServerId + `",
				"isDefault": true
			  }
		  ],
		  "missioncontrol": {
			"url": "http://localhost:8080/mc/"
		  },
		  "version": "4"
		}
	`

	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(configV4))
	assert.NoError(t, err)
	configV6 := new(ConfigV6)
	assert.NoError(t, json.Unmarshal(content, &configV6))
	assertionHelper(t, configV6, 4)
}

func TestConvertConfigV5ToV6(t *testing.T) {

	configV5 := `
		{
		  "servers": [
			      {
					  "url": "http://localhost:8080/",
					  "artifactoryUrl": "http://localhost:8080/artifactory/",
					  "distributionUrl": "http://localhost:8080/distribution/",
					  "xrayUrl": "http://localhost:8080/xray/",
					  "missionControlUrl": "http://localhost:8080/mc/",
					  "pipelinesUrl": "http://localhost:8080/pipelines/",
					  "user": "user",
			          "password": "password",
					  "accessToken": "M9Zi1FY_lpA5dR01ev6EU6Tx_qRVsm2mSYWqobz",
					  "RefreshToken": "a476324f-856c-41d7-b87e-3162e7d6jk91",
					  "serverId": "Default-Server",
  					  "isDefault": true
				  }
		  ],
		  "version": "5"
		}
	`

	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(configV5))
	assert.NoError(t, err)
	configV6 := new(ConfigV6)
	assert.NoError(t, json.Unmarshal(content, &configV6))
	assertionHelper(t, configV6, 5)
}

func TestConfigEncryption(t *testing.T) {
	// Config
	cleanUpTempEnv := configtests.CreateTempEnv(t, true)
	defer cleanUpTempEnv()

	// Original decrypted config, read directly from file
	originalConfig := readConfFromFile(t)

	// Reading through this function will update encryption, and encrypt the config file.
	readConfig, err := readConf()
	assert.NoError(t, err)

	// Config file encryption should be updated, so Enc=true. Secrets should be decrypted to be used in the rest of the execution.
	verifyEncryptionStatus(t, originalConfig, readConfig, false)
	// Config file should be encrypted.
	encryptedConfig := readConfFromFile(t)
	verifyEncryptionStatus(t, originalConfig, encryptedConfig, true)

	// Verify successfully decrypting.
	readConfig, err = readConf()
	assert.NoError(t, err)
	verifyEncryptionStatus(t, originalConfig, readConfig, false)
}

func TestConfigEncryptionEnvVar(t *testing.T) {
	// Config
	cleanUpTempEnv := configtests.CreateTempEnv(t, false)
	defer cleanUpTempEnv()

	// Set encryption key in JFROG_CLI_ENCRYPTION_KEY environment variable
	key := uuid.NewString()[:32]
	assert.NoError(t, os.Setenv(coreutils.EncryptionKey, key))
	defer func() {
		assert.NoError(t, os.Unsetenv(coreutils.EncryptionKey))
	}()

	// Save the config and ensure the secrets were updated
	expectedConfig := createEncryptionTestConfig()
	assert.NoError(t, saveConfig(expectedConfig))
	actualConfig := readConfFromFile(t)
	verifyEncryptionStatus(t, expectedConfig, actualConfig, true)

	// Read config and ensure the server details are decrypted
	actualConfig, err := readConf()
	assert.NoError(t, err)
	verifyEncryptionStatus(t, expectedConfig, actualConfig, false)
}

func TestConfigEncryptionEnvVarUpdate(t *testing.T) {
	// Set testing environment
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	assert.NoError(t, saveConfig(createEncryptionTestConfig()))
	expectedConfig := createEncryptionTestConfig()
	actualConfig := readConfFromFile(t)

	verifyEncryptionStatus(t, expectedConfig, actualConfig, false)

	// Set encryption key in JFROG_CLI_ENCRYPTION_KEY environment variable
	key := uuid.NewString()[:32]
	assert.NoError(t, os.Setenv(coreutils.EncryptionKey, key))
	defer func() {
		assert.NoError(t, os.Unsetenv(coreutils.EncryptionKey))
	}()

	// Read config and ensure that the config was decrypted
	actualConfig, err = readConf()
	assert.NoError(t, err)
	verifyEncryptionStatus(t, expectedConfig, actualConfig, false)
}

func createEncryptionTestConfig() *Config {
	return &Config{ConfigV6{ConfigV5{
		Version: strconv.Itoa(coreutils.GetCliConfigVersion()),
		Servers: []*ServerDetails{{
			ServerId:      "test-server",
			Url:           "http://acme.jfrog.io",
			User:          "elmar",
			Password:      "Wabbit",
			AccessToken:   "DewiciousWegOfWamb",
			SshPassphrase: "KiwwTheWabbit",
		}}},
	}}
}

// Read config file "as-is" - without decryption
func readConfFromFile(t *testing.T) *Config {
	confFilePath, err := getConfFilePath()
	assert.NoError(t, err)
	config := new(Config)
	assert.FileExists(t, confFilePath)
	content, err := fileutils.ReadFile(confFilePath)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(content, &config))
	return config
}

func TestGetArtifactoriesFromConfig(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	config := `
		{
		  "artifactory": [
			{
			  "url": "http://localhost:8080/artifactory/",
			  "user": "user",
			  "password": "password",
			  "serverId": "name",
			  "isDefault": true
			},
			{
			  "url": "http://localhost:8080/artifactory/",
			  "user": "user",
			  "password": "password",
			  "serverId": "notDefault"
			}
		  ],
		  "version": "2"
		}
	`
	content, err := convertIfNeeded([]byte(config))
	assert.NoError(t, err)
	latestConfig := new(Config)
	assert.NoError(t, json.Unmarshal(content, &latestConfig))
	serverDetails, err := GetDefaultConfiguredConf(latestConfig.Servers)
	assert.NoError(t, err)
	assert.Equal(t, serverDetails.ServerId, "name")

	serverDetails, err = getServerConfByServerId("notDefault", latestConfig.Servers)
	assert.NoError(t, err)
	assert.Equal(t, serverDetails.ServerId, "notDefault")
}

func TestGetJfrogDependenciesPath(t *testing.T) {
	// Check default value of dependencies path, should be JFROG_CLI_HOME_DIR/dependencies
	dependenciesPath, err := GetJfrogDependenciesPath()
	assert.NoError(t, err)
	jfrogHomeDir, err := coreutils.GetJfrogHomeDir()
	assert.NoError(t, err)
	expectedDependenciesPath := filepath.Join(jfrogHomeDir, coreutils.JfrogDependenciesDirName)
	assert.Equal(t, expectedDependenciesPath, dependenciesPath)

	// Check dependencies' path when JFROG_DEPENDENCIES_DIR is set, should be JFROG_DEPENDENCIES_DIR/
	previousDependenciesDirEnv := os.Getenv(coreutils.DependenciesDir)
	expectedDependenciesPath = "/tmp/my-dependencies/"
	testsutils.SetEnvAndAssert(t, coreutils.DependenciesDir, expectedDependenciesPath)
	defer testsutils.SetEnvAndAssert(t, coreutils.DependenciesDir, previousDependenciesDirEnv)
	dependenciesPath, err = GetJfrogDependenciesPath()
	assert.NoError(t, err)
	assert.Equal(t, expectedDependenciesPath, dependenciesPath)
}

func assertionV4Helper(t *testing.T, convertedConfig *ConfigV4, expectedVersion int, expectedEnc bool) {
	assert.Equal(t, strconv.Itoa(expectedVersion), convertedConfig.Version)
	assert.Equal(t, expectedEnc, convertedConfig.Enc)

	rtConverted := convertedConfig.Artifactory
	if rtConverted == nil {
		assert.Fail(t, "empty Artifactory config!")
		return
	}
	assert.Len(t, rtConverted, 1)
	rtConfigType := reflect.TypeOf(rtConverted)
	assert.Equal(t, "[]*config.ServerDetails", rtConfigType.String())
	assert.True(t, rtConverted[0].IsDefault)
	assert.Equal(t, DefaultServerId, rtConverted[0].ServerId)
	assert.Equal(t, "http://localhost:8080/artifactory/", rtConverted[0].Url)
	assert.Equal(t, "user", rtConverted[0].User)
	assert.Equal(t, "password", rtConverted[0].Password)
}

func assertionHelper(t *testing.T, convertedConfig *ConfigV6, previousVersion int) {
	assert.Equal(t, "6", convertedConfig.Version)

	serversConverted := convertedConfig.Servers
	if serversConverted == nil {
		assert.Fail(t, "empty servers config!")
		return
	}
	assert.Len(t, serversConverted, 1)
	rtConfigType := reflect.TypeOf(serversConverted)
	assert.Equal(t, "[]*config.ServerDetails", rtConfigType.String())
	assert.True(t, serversConverted[0].IsDefault)
	assert.Equal(t, DefaultServerId, serversConverted[0].ServerId)
	assert.Equal(t, "http://localhost:8080/artifactory/", serversConverted[0].ArtifactoryUrl)
	assert.Equal(t, "http://localhost:8080/mc/", serversConverted[0].MissionControlUrl)
	assert.Equal(t, "user", serversConverted[0].User)
	assert.Equal(t, "password", serversConverted[0].Password)
	if previousVersion >= 5 {
		assert.Equal(t, "http://localhost:8080/xray/", serversConverted[0].XrayUrl)
		assert.Equal(t, "http://localhost:8080/distribution/", serversConverted[0].DistributionUrl)
		assert.Equal(t, "M9Zi1FY_lpA5dR01ev6EU6Tx_qRVsm2mSYWqobz", serversConverted[0].AccessToken)
		assert.Equal(t, "a476324f-856c-41d7-b87e-3162e7d6jk91", serversConverted[0].ArtifactoryRefreshToken)
		assert.Equal(t, "", serversConverted[0].RefreshToken)
	}
}

func TestHandleSecrets(t *testing.T) {
	masterKey := "randomkeywithlengthofexactly32!!"

	original := new(Config)
	original.Servers = []*ServerDetails{{User: "user", Password: "password", Url: "http://localhost:8080/artifactory/", AccessToken: "accessToken",
		RefreshToken: "refreshToken", SshPassphrase: "sshPass"}}
	original.Enc = true

	newConf, err := original.Clone()
	assert.NoError(t, err)

	// Encrypt decrypted
	assert.NoError(t, handleSecrets(original, encrypt, masterKey))
	verifyEncryptionStatus(t, original, newConf, true)

	// Decrypt encrypted
	assert.NoError(t, handleSecrets(original, decrypt, masterKey))
	newConf.Enc = false
	verifyEncryptionStatus(t, original, newConf, false)
}

func verifyEncryptionStatus(t *testing.T, original, actual *Config, encryptionExpected bool) {
	assert.Equal(t, len(original.Servers), len(actual.Servers))
	var equals []bool
	for i := range actual.Servers {
		if original.Servers[i].Password != "" {
			equals = append(equals, original.Servers[i].Password == actual.Servers[i].Password)
		}
		if original.Servers[i].AccessToken != "" {
			equals = append(equals, original.Servers[i].AccessToken == actual.Servers[i].AccessToken)
		}
		if original.Servers[i].RefreshToken != "" {
			equals = append(equals, original.Servers[i].RefreshToken == actual.Servers[i].RefreshToken)
		}
		assert.Equal(t, encryptionExpected, actual.Enc)
	}

	if encryptionExpected {
		// Verify non match.
		assert.Zero(t, coreutils.SumTrueValues(equals))
	} else {
		// Verify all match.
		assert.Equal(t, coreutils.SumTrueValues(equals), len(equals))
	}
}

func assertCertsMigration(t *testing.T) {
	certsDir, err := coreutils.GetJfrogCertsDir()
	assert.NoError(t, err)
	assert.DirExists(t, certsDir)
	files, err := os.ReadDir(certsDir)
	assert.NoError(t, err)
	// Verify only the certs were moved
	assert.Len(t, files, 2)
}
