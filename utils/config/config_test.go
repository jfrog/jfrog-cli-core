package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetDefaultLogger()
}

const certsConversionResources = "testdata/config/configconversion"
const encryptionResources = "testdata/config/encryption"

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

func TestConvertConfigV0ToV5(t *testing.T) {
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

	cleanUpTempEnv := createTempEnv(t)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(configV0))
	assert.NoError(t, err)
	configV5 := new(ConfigV5)
	assert.NoError(t, json.Unmarshal(content, &configV5))
	assertionV5Helper(t, configV5, coreutils.GetConfigVersion(), false)
	assertCertsMigrationAndBackupCreation(t)
}

func TestConvertConfigV1ToV5(t *testing.T) {
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

	cleanUpTempEnv := createTempEnv(t)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(config))
	assert.NoError(t, err)
	configV5 := new(ConfigV5)
	assert.NoError(t, json.Unmarshal(content, &configV5))
	assertionV5Helper(t, configV5, coreutils.GetConfigVersion(), false)

	assert.Equal(t, "user", configV5.Servers[0].User, "The config conversion to version 3 is supposed to save the username as lowercase")

	assertCertsMigrationAndBackupCreation(t)
}

func assertCertsMigrationAndBackupCreation(t *testing.T) {
	assertCertsMigration(t)
	backupDir, err := coreutils.GetJfrogBackupDir()
	assert.NoError(t, err)
	assert.DirExists(t, backupDir)
}

func TestConvertConfigV4ToV5(t *testing.T) {
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

	cleanUpTempEnv := createTempEnv(t)
	defer cleanUpTempEnv()
	content, err := convertIfNeeded([]byte(configV4))
	assert.NoError(t, err)
	configV5 := new(ConfigV5)
	assert.NoError(t, json.Unmarshal(content, &configV5))
	assertionV5Helper(t, configV5, coreutils.GetConfigVersion(), false)
}

func TestConfigEncryption(t *testing.T) {
	// Config
	cleanUpTempEnv := createTempEnv(t)
	defer cleanUpTempEnv()

	// Original decrypted config, read directly from file
	originalConfig := readConfFromFile(t)

	// Reading through this function will update encryption, and encrypt the config file.
	readConfig, err := readConf()
	assert.NoError(t, err)

	// Config file encryption should be updated, so Enc=true. Secrets should be decrypted to be used in the rest of the execution.
	assert.True(t, readConfig.Enc)
	verifyEncryptionStatus(t, originalConfig, readConfig, false)
	// Config file should be encrypted.
	encryptedConfig := readConfFromFile(t)
	verifyEncryptionStatus(t, originalConfig, encryptedConfig, true)

	// Verify successfully decrypting.
	readConfig, err = readConf()
	assert.NoError(t, err)
	assert.True(t, readConfig.Enc)
	verifyEncryptionStatus(t, originalConfig, readConfig, false)
}

func readConfFromFile(t *testing.T) *ConfigV5 {
	confFilePath, err := getConfFilePath()
	assert.NoError(t, err)
	config := new(ConfigV5)
	assert.FileExists(t, confFilePath)
	content, err := fileutils.ReadFile(confFilePath)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(content, &config))
	return config
}

// Set JFROG_CLI_HOME_DIR environment variable to be a new temp directory
func createTempEnv(t *testing.T) (cleanUp func()) {
	tmpDir, err := ioutil.TempDir("", "config_test")
	assert.NoError(t, err)
	oldHome := os.Getenv(coreutils.HomeDir)
	assert.NoError(t, os.Setenv(coreutils.HomeDir, tmpDir))
	copyResources(t, certsConversionResources, tmpDir)
	return func() {
		os.RemoveAll(tmpDir)
		os.Setenv(coreutils.HomeDir, oldHome)
	}
}

func TestGetArtifactoriesFromConfig(t *testing.T) {
	err, cleanUpJfrogHome := tests.SetJfrogHome()
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
	configV5 := new(ConfigV5)
	assert.NoError(t, json.Unmarshal(content, &configV5))
	serverDetails, err := GetDefaultConfiguredConf(configV5.Servers)
	assert.NoError(t, err)
	assert.Equal(t, serverDetails.ServerId, "name")

	serverDetails, err = getServerConfByServerId("notDefault", configV5.Servers)
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

	// Check dependencies path when JFROG_DEPENDENCIES_DIR is set, should be JFROG_DEPENDENCIES_DIR/
	previousDependenciesDirEnv := os.Getenv(coreutils.DependenciesDir)
	expectedDependenciesPath = "/tmp/my-dependencies/"
	err = os.Setenv(coreutils.DependenciesDir, expectedDependenciesPath)
	assert.NoError(t, err)
	defer os.Setenv(coreutils.DependenciesDir, previousDependenciesDirEnv)
	dependenciesPath, err = GetJfrogDependenciesPath()
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

func assertionV5Helper(t *testing.T, convertedConfig *ConfigV5, expectedVersion int, expectedEnc bool) {
	assert.Equal(t, strconv.Itoa(expectedVersion), convertedConfig.Version)
	assert.Equal(t, expectedEnc, convertedConfig.Enc)

	rtConverted := convertedConfig.Servers
	if rtConverted == nil {
		assert.Fail(t, "empty servers config!")
		return
	}
	assert.Len(t, rtConverted, 1)
	rtConfigType := reflect.TypeOf(rtConverted)
	assert.Equal(t, "[]*config.ServerDetails", rtConfigType.String())
	assert.True(t, rtConverted[0].IsDefault)
	assert.Equal(t, DefaultServerId, rtConverted[0].ServerId)
	assert.Equal(t, "http://localhost:8080/artifactory/", rtConverted[0].ArtifactoryUrl)
	assert.Equal(t, "http://localhost:8080/mc/", rtConverted[0].MissionControlUrl)
	assert.Equal(t, "user", rtConverted[0].User)
	assert.Equal(t, "password", rtConverted[0].Password)
}

func TestHandleSecrets(t *testing.T) {
	masterKey := "randomkeywithlengthofexactly32!!"

	original := new(ConfigV5)
	original.Servers = []*ServerDetails{{User: "user", Password: "password", Url: "http://localhost:8080/artifactory/", AccessToken: "accessToken",
		RefreshToken: "refreshToken", SshPassphrase: "sshPass"}}

	newConf := copyConfig(t, original)

	// Encrypt decrypted
	assert.NoError(t, handleSecrets(original, encrypt, masterKey))
	verifyEncryptionStatus(t, original, newConf, true)

	// Decrypt encrypted
	assert.NoError(t, handleSecrets(original, decrypt, masterKey))
	verifyEncryptionStatus(t, original, newConf, false)
}

func copyConfig(t *testing.T, original *ConfigV5) *ConfigV5 {
	b, err := json.Marshal(&original)
	assert.NoError(t, err)
	newConf := new(ConfigV5)
	err = json.Unmarshal(b, &newConf)
	assert.NoError(t, err)
	return newConf
}

func verifyEncryptionStatus(t *testing.T, original, actual *ConfigV5, encryptionExpected bool) {
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
	}

	if encryptionExpected {
		// Verify non match.
		assert.Zero(t, coreutils.SumTrueValues(equals))
	} else {
		// Verify all match.
		assert.Equal(t, coreutils.SumTrueValues(equals), len(equals))
	}
}

func copyResources(t *testing.T, sourcePath string, destPath string) {
	assert.NoError(t, fileutils.CopyDir(sourcePath, destPath, true, nil))
}

func assertCertsMigration(t *testing.T) {
	certsDir, err := coreutils.GetJfrogCertsDir()
	assert.NoError(t, err)
	assert.DirExists(t, certsDir)
	secFile, err := coreutils.GetJfrogSecurityConfFilePath()
	assert.NoError(t, err)
	assert.FileExists(t, secFile)
	files, err := ioutil.ReadDir(certsDir)
	assert.NoError(t, err)
	// Verify only the certs were moved
	assert.Len(t, files, 2)
}
