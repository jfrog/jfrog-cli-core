package transferconfig

import (
	"bytes"
	"encoding/json"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestExportSourceArtifactory(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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

	// Test export source artifactory
	transferConfigCmd := NewTransferConfigCommand(serverDetails, nil)
	exportDir, cleanUp, err := transferConfigCmd.exportSourceArtifactory(serviceManager)
	assert.NoError(t, err)
	assert.DirExists(t, exportDir)
	assert.NoError(t, cleanUp())
	assert.NoDirExists(t, exportDir)
}

func TestImportToTargetArtifactory(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		content, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		if r.RequestURI == "/api/plugins/execute/configImport" {
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
	transferConfigCmd := NewTransferConfigCommand(nil, serverDetails)
	err := transferConfigCmd.importToTargetArtifactory(serviceManager, bytes.NewBuffer([]byte("zip-content")))
	assert.NoError(t, err)
}

func TestGetConfigXml(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/api/system/configuration" {
			w.Write([]byte("<config></config>"))
		}
	})
	defer testServer.Close()

	// Test get config xml
	transferConfigCmd := NewTransferConfigCommand(serverDetails, nil)
	configXml, err := transferConfigCmd.getConfigXml(serviceManager, "7.0.0")
	assert.NoError(t, err)
	assert.Equal(t, "<config></config>", configXml)
}

func TestSanityVerifications(t *testing.T) {
	users := []services.User{}
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/plugins/execute/checkPermissions" {
			w.WriteHeader(http.StatusOK)
		} else if r.RequestURI == "/api/plugins/execute/configImportVersion" {
			content, err := json.Marshal(commandUtils.VersionResponse{Version: "1.0.0"})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		} else {
			content, err := json.Marshal(users)
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
			users = append(users, services.User{})
		}
	})
	defer testServer.Close()
	transferConfigCmd := NewTransferConfigCommand(&config.ServerDetails{Url: "dummy-url"}, serverDetails)

	// Test low artifactory version
	err := transferConfigCmd.validateArtifactoryServers(serviceManager, "6.0.0")
	assert.ErrorContains(t, err, "This operation requires source Artifactory version 6.23.21 or higher")

	// Test no users
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.NoError(t, err)

	// Test 1 users
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.NoError(t, err)

	// Test 2 users
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.NoError(t, err)

	// Test 3 users
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.ErrorContains(t, err, "cowardly refusing to import the config to the target server, because it contains more than 2 users.")

	// Assert force = true
	transferConfigCmd.force = true
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.NoError(t, err)

	// Test same source and target Artifactory servers
	transferConfigCmd = NewTransferConfigCommand(serverDetails, serverDetails)
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.ErrorContains(t, err, "The source and target Artifactory servers are identical, but should be different.")

}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(&config.ServerDetails{Url: "dummy-url"}, serverDetails)
	err := transferConfigCmd.verifyConfigImportPlugin(serviceManager)
	assert.ErrorContains(t, err, "Response from Artifactory: 404 Not Found.")
}

func TestVerifyConfigImportPluginForbidden(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("An admin user is required"))
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(&config.ServerDetails{Url: "dummy-url"}, serverDetails)
	err := transferConfigCmd.verifyConfigImportPlugin(serviceManager)
	assert.ErrorContains(t, err, "Response from Artifactory: 403 Forbidden.")
}

func TestIsDefaultCredentialsDefault(t *testing.T) {
	unlockCounter := 0
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("OK"))
			assert.NoError(t, err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(serverDetails, &config.ServerDetails{Url: "dummy-url"})
	isDefaultCreds, err := transferConfigCmd.isDefaultCredentials(serviceManager)
	assert.NoError(t, err)
	assert.True(t, isDefaultCreds)
	assert.Equal(t, 0, unlockCounter)
}

func TestIsDefaultCredentialsNotDefault(t *testing.T) {
	unlockCounter := 0
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(serverDetails, &config.ServerDetails{Url: "dummy-url"})
	isDefaultCreds, err := transferConfigCmd.isDefaultCredentials(serviceManager)
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 1, unlockCounter)
}

func TestIsDefaultCredentialsLocked(t *testing.T) {
	pingCounter := 0
	unlockCounter := 0
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[ \"admin\" ]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
			pingCounter++
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(serverDetails, &config.ServerDetails{Url: "dummy-url"})
	isDefaultCreds, err := transferConfigCmd.isDefaultCredentials(serviceManager)
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 0, pingCounter)
	assert.Equal(t, 0, unlockCounter)
}
