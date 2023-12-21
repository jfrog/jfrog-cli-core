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

const (
	repoKey    = "repoKey"
	projectKey = "test-proj"
)

var transferConfigTestDir = filepath.Join("testdata", "transferconfig")

func TestIsDefaultCredentialsDefault(t *testing.T) {
	unlockCounter := 0
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/api/security/lockedUsers":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		case "/api/system/ping":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("OK"))
			assert.NoError(t, err)
		default:
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
		switch r.RequestURI {
		case "/api/security/lockedUsers":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		case "/api/system/ping":
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
		default:
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
		switch r.RequestURI {
		case "/api/security/lockedUsers":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[ \"admin\" ]"))
			assert.NoError(t, err)
		case "/api/system/ping":
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
			pingCounter++
		default:
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

func TestValidateDifferentServers(t *testing.T) {
	var sourceRtVersion string
	// Create transfer config command
	_, sourceServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		content, err := json.Marshal(VersionResponse{Version: sourceRtVersion})
		assert.NoError(t, err)
		_, err = w.Write(content)
		assert.NoError(t, err)
	})

	t.Run("Same source and target servers", func(t *testing.T) {
		err := createTransferConfigBase(t, sourceServerDetails, sourceServerDetails).ValidateDifferentServers()
		assert.ErrorContains(t, err, "The source and target Artifactory servers are identical, but should be different.")
	})
}

func TestGetSelectedRepositories(t *testing.T) {
	sourceTestServer, sourceServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		repositories := &[]services.RepositoryDetails{
			{Key: "generic-local", Type: "local"}, {Key: "generic-local-filter", Type: "local"}, {Key: "generic-local-existed", Type: "local"},
			{Key: "generic-remote", Type: "remote"}, {Key: "generic-filter-remote", Type: "remote"},
			{Key: "generic-virtual", Type: "virtual"}, {Key: "filter-generic-virtual", Type: "virtual"},
			{Key: "generic-federated", Type: "federated"}, {Key: "generic-federated-filter", Type: "federated"},
		}
		reposBytes, err := json.Marshal(repositories)
		assert.NoError(t, err)
		_, err = w.Write(reposBytes)
		assert.NoError(t, err)
	})
	defer sourceTestServer.Close()
	targetTestServer, targetServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		repositories := &[]services.RepositoryDetails{{Key: "generic-local-existed", Type: "local"}}
		reposBytes, err := json.Marshal(repositories)
		assert.NoError(t, err)
		_, err = w.Write(reposBytes)
		assert.NoError(t, err)
	})
	defer targetTestServer.Close()

	transferConfigBase := createTransferConfigBase(t, sourceServerDetails, targetServerDetails)
	transferConfigBase.SetExcludeReposPatterns([]string{"*filter*"})
	selectedRepos, err := transferConfigBase.GetSelectedRepositories()
	assert.NoError(t, err)
	assert.Len(t, selectedRepos, 4)
	assert.Equal(t, []services.RepositoryDetails{{Key: "generic-local", Type: "local"}}, selectedRepos[utils.Local])
	assert.Equal(t, []services.RepositoryDetails{{Key: "generic-remote", Type: "remote"}}, selectedRepos[utils.Remote])
	assert.Equal(t, []services.RepositoryDetails{{Key: "generic-virtual", Type: "virtual"}}, selectedRepos[utils.Virtual])
	assert.Equal(t, []services.RepositoryDetails{{Key: "generic-federated", Type: "federated"}}, selectedRepos[utils.Federated])
}

func TestTransferRepositoryToTarget(t *testing.T) {
	federatedRepo := readRepoConfig(t, "federated_repo")
	federatedRepoWithoutMembers := readRepoConfig(t, "federated_repo_without_members")

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.Method {
		case http.MethodGet:
			switch r.RequestURI {
			case "/api/repositories/federated-local":
				_, err := w.Write(federatedRepo)
				assert.NoError(t, err)
			case "/api/repositories/federated-local-no-members":
				_, err := w.Write(federatedRepoWithoutMembers)
				assert.NoError(t, err)
			default:
				assert.Fail(t, "Unexpected request URI "+r.RequestURI)
			}
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, getRepoParamsMap(t, federatedRepoWithoutMembers), getRepoParamsMap(t, body))
		default:
			assert.Fail(t, "Unexpected method "+r.Method)
		}
	})
	defer testServer.Close()

	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)
	assert.False(t, transferConfigBase.FederatedMembersRemoved)
	err := transferConfigBase.transferSpecificRepositoriesToTarget([]services.RepositoryDetails{
		{Key: "federated-local"}, {Key: "federated-local-no-members"}}, utils.Federated)
	assert.NoError(t, err)
	assert.True(t, transferConfigBase.FederatedMembersRemoved)
}

func TestTransferVirtualRepositoriesToTarget(t *testing.T) {
	virtualRepoA := readRepoConfig(t, "virtual_repo_a")
	virtualRepoB := readRepoConfig(t, "virtual_repo_b")

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.Method {
		case http.MethodGet:
			switch r.RequestURI {
			case "/api/repositories/a-virtual":
				_, err := w.Write(virtualRepoA)
				assert.NoError(t, err)
			case "/api/repositories/b-virtual":
				_, err := w.Write(virtualRepoB)
				assert.NoError(t, err)
			default:
				assert.Fail(t, "Unexpected request URI "+r.RequestURI)
			}
		case http.MethodPut, http.MethodPost:
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			expectedVirtualRepoAParamsMap := getRepoParamsMap(t, virtualRepoA)
			expectedVirtualRepoBParamsMap := getRepoParamsMap(t, virtualRepoB)

			if r.Method == http.MethodPut {
				delete(expectedVirtualRepoAParamsMap, "repositories")
				delete(expectedVirtualRepoBParamsMap, "repositories")
			}

			switch r.RequestURI {
			case "/api/repositories/a-virtual":
				assert.Equal(t, expectedVirtualRepoAParamsMap, getRepoParamsMap(t, body))
			case "/api/repositories/b-virtual":
				assert.Equal(t, expectedVirtualRepoBParamsMap, getRepoParamsMap(t, body))
			default:
				assert.Fail(t, "Unexpected request URI "+r.RequestURI)
			}
		}
	})
	defer testServer.Close()

	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)
	assert.NoError(t, transferConfigBase.transferVirtualRepositoriesToTarget([]services.RepositoryDetails{{Key: "a-virtual"}, {Key: "b-virtual"}}))
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

var removeProjectKeyCases = []struct {
	repoKey            string
	projectKey         string
	expectedProjectKey string
}{
	{repoKey: repoKey, projectKey: "", expectedProjectKey: ""},
	{repoKey: repoKey, projectKey: "default", expectedProjectKey: ""},
	{repoKey: repoKey, projectKey: projectKey, expectedProjectKey: projectKey},
	{repoKey: projectKey + "-" + repoKey, projectKey: projectKey, expectedProjectKey: ""},
}

func TestRemoveProjectKey(t *testing.T) {
	repoParams := services.NewLocalRepositoryBaseParams()

	for _, testCase := range removeProjectKeyCases {
		t.Run(testCase.repoKey, func(t *testing.T) {
			repoParams.ProjectKey = testCase.projectKey
			actualRepoParams, actualProjectKey, err := removeProjectKeyIfNeeded(repoParams, testCase.repoKey)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedProjectKey, actualProjectKey)
			if testCase.expectedProjectKey == "" {
				assert.Equal(t, repoParams, actualRepoParams)
			} else {
				assert.NotEqual(t, repoParams, actualRepoParams)
			}
		})
	}
}

func TestRemoveProjectKeyIllegal(t *testing.T) {
	type illegalRepoParamsStruct struct {
		ProjectKey int `json:"projectKey"`
	}
	illegalRepoParams := &illegalRepoParamsStruct{ProjectKey: 7}
	actualRepoParams, projectKey, err := removeProjectKeyIfNeeded(illegalRepoParams, repoKey)
	assert.ErrorContains(t, err, "couldn't parse the 'projectKey' value '7' of repository 'repoKey'")
	assert.Empty(t, projectKey)
	assert.Nil(t, actualRepoParams)
}

func TestCreateRepositoryAndAssignToProject(t *testing.T) {
	projectUnassigned := false
	repositoryCreated := false
	projectAssigned := false
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/access/api/v1/projects/_/attach/repositories/local-repo":
			projectUnassigned = true
		case "/api/repositories/local-repo":
			repositoryCreated = true
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			_, exist := getRepoParamsMap(t, body)["projectKey"]
			assert.False(t, exist)
		case "/access/api/v1/projects/_/attach/repositories/local-repo/test-proj?force=true":
			projectAssigned = true
		default:
			assert.Fail(t, "Unexpected request URI: "+r.RequestURI)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer testServer.Close()
	transferConfigBase := createTransferConfigBase(t, serverDetails, serverDetails)

	repoParams := services.NewLocalRepositoryBaseParams()
	repoParams.Key = "local-repo"
	repoParams.ProjectKey = projectKey
	err := transferConfigBase.createRepositoryAndAssignToProject(repoParams, services.RepositoryDetails{Key: repoParams.Key})
	assert.NoError(t, err)
	assert.True(t, projectUnassigned)
	assert.True(t, repositoryCreated)
	assert.True(t, projectAssigned)
}

func createTransferConfigBase(t *testing.T, sourceServerDetails, targetServerDetails *config.ServerDetails) *TransferConfigBase {
	transferConfigBase := NewTransferConfigBase(sourceServerDetails, targetServerDetails)
	assert.NoError(t, transferConfigBase.CreateServiceManagers(false))
	return transferConfigBase
}

func getRepoParamsMap(t *testing.T, body []byte) map[string]interface{} {
	var repoParams interface{}
	assert.NoError(t, json.Unmarshal(body, &repoParams))
	repoParamsMap, err := InterfaceToMap(repoParams)
	assert.NoError(t, err)
	return repoParamsMap
}

func readRepoConfig(t *testing.T, fileName string) []byte {
	repoConfig, err := fileutils.ReadFile(filepath.Join(transferConfigTestDir, fileName+".json"))
	assert.NoError(t, err)
	return repoConfig
}
