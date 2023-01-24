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
	transferConfigCmd := NewTransferConfigCommand(nil, serverDetails)
	err := transferConfigCmd.importToTargetArtifactory(serviceManager, bytes.NewBuffer([]byte("zip-content")))
	assert.NoError(t, err)
}

func TestGetConfigXml(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/api/system/configuration" {
			_, err := w.Write([]byte("<config></config>"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	// Test get config xml
	transferConfigCmd := NewTransferConfigCommand(serverDetails, nil)
	configXml, err := transferConfigCmd.getConfigXml(serviceManager)
	assert.NoError(t, err)
	assert.Equal(t, "<config></config>", configXml)
}

func TestSanityVerifications(t *testing.T) {
	var users []services.User
	var rtVersion string
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+commandUtils.PluginsExecuteRestApi+"checkPermissions" {
			w.WriteHeader(http.StatusOK)
		} else if r.RequestURI == "/"+commandUtils.PluginsExecuteRestApi+"configImportVersion" {
			content, err := json.Marshal(commandUtils.VersionResponse{Version: "1.0.0"})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/version" {
			content, err := json.Marshal(commandUtils.VersionResponse{Version: rtVersion})
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

	// Test low Artifactory version
	rtVersion = "6.0.0"
	_, err := validateMinVersionAndDifferentServers(serviceManager, serverDetails, serverDetails)
	assert.ErrorContains(t, err, "while this operation requires version")

	// Test same source and target Artifactory servers
	rtVersion = minArtifactoryVersion
	_, err = validateMinVersionAndDifferentServers(serviceManager, serverDetails, serverDetails)
	assert.ErrorContains(t, err, "The source and target Artifactory servers are identical, but should be different.")

	transferConfigCmd := NewTransferConfigCommand(&config.ServerDetails{Url: "dummy-url"}, serverDetails)
	// Test no users
	err = transferConfigCmd.validateTargetServer(serviceManager)
	assert.NoError(t, err)

	// Test 1 users
	err = transferConfigCmd.validateTargetServer(serviceManager)
	assert.NoError(t, err)

	// Test 2 users
	err = transferConfigCmd.validateTargetServer(serviceManager)
	assert.NoError(t, err)

	// Test 3 users
	err = transferConfigCmd.validateTargetServer(serviceManager)
	assert.ErrorContains(t, err, "cowardly refusing to import the config to the target server, because it contains more than 2 users.")

	// Assert force = true
	transferConfigCmd.force = true
	err = transferConfigCmd.validateTargetServer(serviceManager)
	assert.NoError(t, err)
}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("Not found"))
		assert.NoError(t, err)
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
		_, err := w.Write([]byte("An admin user is required"))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(&config.ServerDetails{Url: "dummy-url"}, serverDetails)
	err := transferConfigCmd.verifyConfigImportPlugin(serviceManager)
	assert.ErrorContains(t, err, "Response from Artifactory: 403 Forbidden.")
}
