package transferconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func TestExportSourceArtifactory(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		// Read body
		content, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var actual services.ExportBody
		assert.NoError(t, json.Unmarshal(content, &actual))

		// Make sure all parameters as expected
		assert.False(t, *actual.IncludeMetadata)
		assert.False(t, *actual.Verbose)
		assert.True(t, *actual.ExcludeContent)
		assert.Nil(t, actual.CreateArchive)
		assert.Nil(t, actual.M2)

		// Create the export-dir in the export path
		assert.NoError(t, os.Mkdir(filepath.Join(actual.ExportPath, "export-dir"), os.ModePerm))
	})
	defer testServer.Close()

	// Create working dir
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()

	// Test export source artifactory
	transferConfigCmd := createTransferConfigCommand(t, serverDetails, nil).SetSourceWorkingDir(tmpDir)
	exportDir, cleanUp, err := transferConfigCmd.exportSourceArtifactory()
	assert.NoError(t, err)
	assert.DirExists(t, exportDir)
	assert.NoError(t, cleanUp())
	assert.NoDirExists(t, exportDir)
	assert.True(t, strings.HasPrefix(exportDir, tmpDir))
}

func TestImportToTargetArtifactory(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		content, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		if r.RequestURI == "/"+commandUtils.PluginsExecuteRestApi+"configImport" {
			// Read body
			assert.Equal(t, []byte("zip-content"), content)
			_, err = w.Write([]byte("123456"))
			assert.NoError(t, err)
			return
		}

		assert.Equal(t, []byte("123456"), content)
	})
	defer testServer.Close()

	// Test export source artifactory
	transferConfigCmd := createTransferConfigCommand(t, nil, serverDetails)
	err := transferConfigCmd.importToTargetArtifactory(bytes.NewBuffer([]byte("zip-content")))
	assert.NoError(t, err)
}

func TestGetConfigXml(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/api/system/configuration" {
			_, err := w.Write([]byte("<config></config>"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	// Test get config xml
	transferConfigCmd := createTransferConfigCommand(t, serverDetails, nil)
	configXml, _, err := transferConfigCmd.getEncryptedItems(make(map[utils.RepoType][]services.RepositoryDetails))
	assert.NoError(t, err)
	assert.Equal(t, "<config></config>", configXml)
}

func TestValidateTargetServer(t *testing.T) {
	var users []services.User
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/" + commandUtils.PluginsExecuteRestApi + "checkPermissions":
			w.WriteHeader(http.StatusOK)
		case "/" + commandUtils.PluginsExecuteRestApi + "configImportVersion":
			content, err := json.Marshal(commandUtils.VersionResponse{Version: "1.0.0"})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		case "/api/system/version":
			content, err := json.Marshal(commandUtils.VersionResponse{Version: "7.0.0"})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		default:
			content, err := json.Marshal(users)
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
			users = append(users, services.User{})
		}
	})
	defer testServer.Close()

	transferConfigCmd := createTransferConfigCommand(t, &config.ServerDetails{Url: "dummy-url"}, serverDetails)
	// Test no users
	err := transferConfigCmd.validateTargetServer()
	assert.NoError(t, err)

	// Test 1 users
	err = transferConfigCmd.validateTargetServer()
	assert.NoError(t, err)

	// Test 2 users
	err = transferConfigCmd.validateTargetServer()
	assert.NoError(t, err)

	// Test 3 users
	err = transferConfigCmd.validateTargetServer()
	assert.ErrorContains(t, err, "cowardly refusing to import the config to the target server, because it contains more than 2 users.")

	// Assert force = true
	transferConfigCmd.force = true
	err = transferConfigCmd.validateTargetServer()
	assert.NoError(t, err)
}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("Not found"))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	transferConfigCmd := createTransferConfigCommand(t, &config.ServerDetails{Url: "dummy-url"}, serverDetails)
	err := transferConfigCmd.verifyConfigImportPlugin()
	assert.ErrorContains(t, err, "Response from Artifactory: 404 Not Found.")
}

func TestVerifyConfigImportPluginForbidden(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, err := w.Write([]byte("An admin user is required"))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	transferConfigCmd := createTransferConfigCommand(t, &config.ServerDetails{Url: "dummy-url"}, serverDetails)
	err := transferConfigCmd.verifyConfigImportPlugin()
	assert.ErrorContains(t, err, "Response from Artifactory: 403 Forbidden.")
}

func TestCreateExportPath(t *testing.T) {
	transferConfigBase := NewTransferConfigCommand(nil, nil)

	// Create export path and check results
	exportPath, unsetTempDir, err := transferConfigBase.createExportPath()
	unsetTempDir()
	assert.NoError(t, err)
	assert.NotEmpty(t, exportPath)
	assert.DirExists(t, exportPath)

	// Create working dir
	const testExportDir = "test-export-dir"
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	tmpDir = filepath.Join(tmpDir, testExportDir)
	assert.NoError(t, os.MkdirAll(tmpDir, 0700))
	transferConfigBase.sourceWorkingDir = tmpDir

	// Create export path with custom working dir
	exportPath, unsetTempDir, err = transferConfigBase.createExportPath()
	unsetTempDir()
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(exportPath, tmpDir))
	assert.DirExists(t, exportPath)

	// Ensure unsetTempDir did work
	assert.NotContains(t, fileutils.GetTempDirBase(), testExportDir)
}

func createTransferConfigCommand(t *testing.T, sourceServerDetails, targetServerDetails *config.ServerDetails) *TransferConfigCommand {
	transferConfigBase := NewTransferConfigCommand(sourceServerDetails, targetServerDetails)
	var err error
	if sourceServerDetails != nil {
		transferConfigBase.SourceArtifactoryManager, err = utils.CreateServiceManager(sourceServerDetails, -1, 0, false)
		assert.NoError(t, err)
	}
	if targetServerDetails != nil {
		transferConfigBase.TargetArtifactoryManager, err = utils.CreateServiceManager(targetServerDetails, -1, 0, false)
		assert.NoError(t, err)
	}
	return transferConfigBase
}

var validateMinVersionAndDifferentServersCases = []struct {
	testName      string
	sourceVersion string
	targetVersion string
	expectedError string
}{
	{testName: "Same version", sourceVersion: minTransferConfigArtifactoryVersion, targetVersion: minTransferConfigArtifactoryVersion, expectedError: ""},
	{testName: "Different versions", sourceVersion: "7.0.0", targetVersion: "7.0.1", expectedError: ""},
	{testName: "Low Artifactory version", sourceVersion: "6.0.0", targetVersion: "7.0.0", expectedError: "while this operation requires version"},
	{testName: "Source newer than target", sourceVersion: "7.0.1", targetVersion: "7.0.0", expectedError: "can't be higher than the target Artifactory version"},
}

func TestValidateMinVersion(t *testing.T) {
	var sourceRtVersion, targetRtVersion string
	// Create transfer config command
	sourceTestServer, sourceServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		content, err := json.Marshal(commandUtils.VersionResponse{Version: sourceRtVersion})
		assert.NoError(t, err)
		_, err = w.Write(content)
		assert.NoError(t, err)
	})
	defer sourceTestServer.Close()
	targetTestServer, targetServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		content, err := json.Marshal(commandUtils.VersionResponse{Version: targetRtVersion})
		assert.NoError(t, err)
		_, err = w.Write(content)
		assert.NoError(t, err)
	})
	defer targetTestServer.Close()

	for _, testCase := range validateMinVersionAndDifferentServersCases {
		t.Run(testCase.testName, func(t *testing.T) {
			sourceRtVersion = testCase.sourceVersion
			targetRtVersion = testCase.targetVersion
			actualSourceRtVersion, err := createTransferConfigCommand(t, sourceServerDetails, targetServerDetails).validateMinVersion()
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, sourceRtVersion, actualSourceRtVersion)
			} else {
				assert.ErrorContains(t, err, testCase.expectedError)
			}
		})
	}
}

func TestValidateAccessServerConnection(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, accessManager := commonTests.CreateAccessRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/access/api/v1/system/ping":
			w.WriteHeader(http.StatusOK)
		default:
			assert.Fail(t, "Unexpected request URI: "+r.RequestURI)
		}
	})
	defer testServer.Close()

	transferConfigCmd := createTransferConfigCommand(t, nil, nil)
	err := transferConfigCmd.ValidateAccessServerConnection(serverDetails, accessManager)
	assert.NoError(t, err)
}

func TestValidateAccessServerConnectionForbidden(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, accessManager := commonTests.CreateAccessRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/access/api/v1/system/ping":
			w.WriteHeader(http.StatusForbidden)
		default:
			assert.Fail(t, "Unexpected request URI: "+r.RequestURI)
		}
	})
	defer testServer.Close()

	transferConfigCmd := createTransferConfigCommand(t, nil, nil)
	// Assert access token invalid error
	err := transferConfigCmd.ValidateAccessServerConnection(serverDetails, accessManager)
	assert.ErrorContains(t, err, "the 'test-server' instance Access Token is not valid. Please provide a valid access token by running the 'jf c edit test-server'")
}
