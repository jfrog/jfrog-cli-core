package utils

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/stretchr/testify/assert"
)

var transferConfigTestDir = filepath.Join("testdata", "transferconfig")

func TestIsDefaultCredentialsDefault(t *testing.T) {
	unlockCounter := 0
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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

	isDefaultCreds, err := createTransferConfigBase(t, serverDetails, serverDetails).IsDefaultCredentials()
	assert.NoError(t, err)
	assert.True(t, isDefaultCreds)
	assert.Equal(t, 0, unlockCounter)
}

func TestIsDefaultCredentialsNotDefault(t *testing.T) {
	unlockCounter := 0
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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

	isDefaultCreds, err := createTransferConfigBase(t, serverDetails, serverDetails).IsDefaultCredentials()
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 1, unlockCounter)
}

func TestIsDefaultCredentialsLocked(t *testing.T) {
	pingCounter := 0
	unlockCounter := 0
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
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

	isDefaultCreds, err := createTransferConfigBase(t, serverDetails, serverDetails).IsDefaultCredentials()
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 0, pingCounter)
	assert.Equal(t, 0, unlockCounter)
}

func TestValidateMinVersionAndDifferentServers(t *testing.T) {
	var rtVersion string
	// Create transfer config command
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		content, err := json.Marshal(VersionResponse{Version: rtVersion})
		assert.NoError(t, err)
		_, err = w.Write(content)
		assert.NoError(t, err)
	})
	defer testServer.Close()

	// Test low Artifactory version
	rtVersion = "6.0.0"
	_, err := createTransferConfigBase(t, serverDetails, serverDetails).ValidateMinVersionAndDifferentServers()
	assert.ErrorContains(t, err, "while this operation requires version")

	// Test same source and target Artifactory servers
	rtVersion = minTransferConfigArtifactoryVersion
	_, err = createTransferConfigBase(t, serverDetails, serverDetails).ValidateMinVersionAndDifferentServers()
	assert.ErrorContains(t, err, "The source and target Artifactory servers are identical, but should be different.")

	// Positive test
	actualVersion, err := createTransferConfigBase(t, serverDetails, &config.ServerDetails{ArtifactoryUrl: "some-different-url"}).ValidateMinVersionAndDifferentServers()
	assert.NoError(t, err)
	assert.Equal(t, rtVersion, actualVersion)
}

func TestGetSelectedRepositories(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		repositories := &[]services.RepositoryDetails{
			{Key: "generic-local", Type: "local"}, {Key: "generic-local-filter", Type: "local"},
			{Key: "generic-remote", Type: "remote"}, {Key: "generic-filter-remote", Type: "remote"},
			{Key: "generic-virtual", Type: "virtual"}, {Key: "filter-generic-virtual", Type: "virtual"},
			{Key: "generic-federated", Type: "federated"}, {Key: "generic-federated-filter", Type: "federated"},
		}
		reposBytes, err := json.Marshal(repositories)
		assert.NoError(t, err)
		_, err = w.Write(reposBytes)
		assert.NoError(t, err)
	})
	defer testServer.Close()

	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)
	transferConfigBase.SetExcludeReposPatterns([]string{"*filter*"})
	selectedRepos, err := transferConfigBase.GetSelectedRepositories()
	assert.NoError(t, err)
	assert.Len(t, selectedRepos, 4)
	assert.Equal(t, []string{"generic-local"}, selectedRepos[utils.Local])
	assert.Equal(t, []string{"generic-remote"}, selectedRepos[utils.Remote])
	assert.Equal(t, []string{"generic-virtual"}, selectedRepos[utils.Virtual])
	assert.Equal(t, []string{"generic-federated"}, selectedRepos[utils.Federated])
}

func TestTransferRepositoryToTarget(t *testing.T) {
	federatedRepo, err := fileutils.ReadFile(filepath.Join(transferConfigTestDir, "federated_repo.json"))
	assert.NoError(t, err)
	federatedRepoWithoutMembers, err := fileutils.ReadFile(filepath.Join(transferConfigTestDir, "federated_repo_without_members.json"))
	assert.NoError(t, err)

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" {
			if r.RequestURI == "/api/repositories/federated-local" {
				_, err := w.Write(federatedRepo)
				assert.NoError(t, err)
			} else if r.RequestURI == "/api/repositories/federated-local-no-members" {
				_, err := w.Write(federatedRepoWithoutMembers)
				assert.NoError(t, err)
			}
		} else if r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var repoParams interface{}
			assert.NoError(t, json.Unmarshal(body, &repoParams))
			repoParamsMap, err := InterfaceToMap(repoParams)
			assert.NoError(t, err)
			assert.Equal(t, getExpectedFederatedRepoParamsMap(t, federatedRepoWithoutMembers), repoParamsMap)
		}
	})
	defer testServer.Close()

	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)
	assert.False(t, transferConfigBase.FederatedMembersRemoved)
	err = transferConfigBase.transferSpecificRepositoriesToTarget([]string{"federated-local", "federated-local-no-members"}, utils.Federated)
	assert.NoError(t, err)
	assert.True(t, transferConfigBase.FederatedMembersRemoved)
}

func TestDeactivateKeyEncryption(t *testing.T) {
	testDeactivateKeyEncryption(t, true)
}

func TestDeactivateKeyEncryptionNotEncrypted(t *testing.T) {
	testDeactivateKeyEncryption(t, false)
}

func testDeactivateKeyEncryption(t *testing.T, wasEncrypted bool) {
	decrypted := false
	reactivated := false
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/system/decrypt" {
			if wasEncrypted {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusConflict)
			}
			decrypted = true
		} else if r.RequestURI == "/api/system/encrypt" {
			reactivated = true
			w.WriteHeader(http.StatusOK)
		}
	})
	defer testServer.Close()
	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)

	reactivate, err := transferConfigBase.DeactivateKeyEncryption()
	assert.NoError(t, err)
	assert.True(t, decrypted)

	assert.False(t, reactivated)
	assert.NoError(t, reactivate())
	assert.Equal(t, reactivated, wasEncrypted)
}

func createTransferConfigBase(t *testing.T, sourceServerDetails, targetServerDetails *config.ServerDetails) *TransferConfigBase {
	transferConfigBase := NewTransferConfigBase(sourceServerDetails, targetServerDetails)
	assert.NoError(t, transferConfigBase.CreateServiceManagers(false))
	return transferConfigBase
}

func getExpectedFederatedRepoParamsMap(t *testing.T, federatedRepoWithoutMembers []byte) map[string]interface{} {
	var expectedRepoParams interface{}
	assert.NoError(t, json.Unmarshal(federatedRepoWithoutMembers, &expectedRepoParams))
	expectedRepoParamsMap, err := InterfaceToMap(expectedRepoParams)
	assert.NoError(t, err)
	return expectedRepoParamsMap
}
