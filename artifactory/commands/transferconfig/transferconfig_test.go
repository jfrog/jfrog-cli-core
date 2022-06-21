package transferconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
)

type tranaferConfigHandler func(w http.ResponseWriter, r *http.Request)

func TestExportSourceArtifactory(t *testing.T) {
	// Create transfer config command
	testServer, serverDetails, serviceManager := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		// Read body
		content, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var actual services.ExportBody
		assert.NoError(t, json.Unmarshal(content, &actual))

		// Make sure all parameters as expected
		assert.True(t, *actual.IncludeMetadata)
		assert.True(t, *actual.Verbose)
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
	testServer, serverDetails, serviceManager := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	testServer, serverDetails, serviceManager := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	testServer, serverDetails, serviceManager := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		content, err := json.Marshal(users)
		assert.NoError(t, err)
		w.Write(content)
		users = append(users, services.User{})
	})
	defer testServer.Close()

	transferConfigCmd := NewTransferConfigCommand(serverDetails, &config.ServerDetails{})

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
	assert.ErrorContains(t, err, "cowardly refusing to import the config to the target server, because it contains more than 2 users. You can bypass this rule by providing the --force flag")

	// Assert force = true
	transferConfigCmd.force = true
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.NoError(t, err)

	// Test same source and target Artifactory servers
	transferConfigCmd = NewTransferConfigCommand(serverDetails, serverDetails)
	err = transferConfigCmd.validateArtifactoryServers(serviceManager, minArtifactoryVersion)
	assert.ErrorContains(t, err, "The source and target Artifactory servers are identical, but should be different.")

}

// Create mock server to test replication body
// t           - The testing object
// testHandler - The HTTP handler of the test
func createMockServer(t *testing.T, testHandler tranaferConfigHandler) (*httptest.Server, *config.ServerDetails, artifactory.ArtifactoryServicesManager) {
	testServer := httptest.NewServer(http.HandlerFunc(testHandler))
	serverDetails := &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}
