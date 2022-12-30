package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

type transferFilesHandler func(w http.ResponseWriter, r *http.Request)

const runningNodesResponse = `
{
	"isHa": true,
	"nodes": [
	  {
		"id": "node-1",
		"state": "RUNNING"
	  },
	  {
		"id": "node-2",
		"state": "RUNNING"
	  },
	  {
		"id": "node-3",
		"state": "RUNNING"
	  }
	]
  }
`

func TestGetRunningNodes(t *testing.T) {
	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(runningNodesResponse))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	runningNodes, err := getRunningNodes(context.Background(), serverDetails)
	assert.NoError(t, err)
	assert.ElementsMatch(t, runningNodes, []string{"node-1", "node-2", "node-3"})
}

func TestStopTransferOnArtifactoryNodes(t *testing.T) {
	stoppedNodeOne, stoppedNodeTwo := false, false
	requestNumber := 0
	testServer, _, srcUpService := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		var nodeId string
		if requestNumber == 0 {
			nodeId = "node-1"
			stoppedNodeOne = true
		} else {
			nodeId = "node-2"
			stoppedNodeTwo = true
		}
		_, err := w.Write([]byte(fmt.Sprintf(`{"node_id": "%s"}`, nodeId)))
		assert.NoError(t, err)
		requestNumber++
	})
	defer testServer.Close()

	stopTransferInArtifactoryNodes(srcUpService, []string{"node-1", "node-2"})
	assert.True(t, stoppedNodeOne)
	assert.True(t, stoppedNodeTwo)
}

const repoConfigurationResponse = `
{
  "key" : "%[1]s-local",
  "packageType" : "%[1]s",
  "description" : "",
  "notes" : "",
  "includesPattern" : "**/*",
  "excludesPattern" : "",
  "repoLayoutRef" : "simple-default",
  "enableComposerSupport" : false,
  "enableNuGetSupport" : false,
  "enableGemsSupport" : false,
  "enableNpmSupport" : false,
  "enableBowerSupport" : false,
  "enableCocoaPodsSupport" : false,
  "enableConanSupport" : false,
  "enableDebianSupport" : false,
  "debianTrivialLayout" : false,
  "enablePypiSupport" : false,
  "enablePuppetSupport" : false,
  "enableDockerSupport" : false,
  "dockerApiVersion" : "V2",
  "blockPushingSchema1" : true,
  "forceNugetAuthentication" : false,
  "enableVagrantSupport" : false,
  "enableGitLfsSupport" : false,
  "enableDistRepoSupport" : false,
  "priorityResolution" : false,
  "checksumPolicyType" : "client-checksums",
  "handleReleases" : true,
  "handleSnapshots" : true,
  "maxUniqueSnapshots" : %[2]d,
  "maxUniqueTags" : %[3]d,
  "snapshotVersionBehavior" : "unique",
  "suppressPomConsistencyChecks" : true,
  "blackedOut" : false,
  "propertySets" : [ ],
  "archiveBrowsingEnabled" : false,
  "calculateYumMetadata" : false,
  "enableFileListsIndexing" : false,
  "yumRootDepth" : 0,
  "downloadRedirect" : false,
  "xrayIndex" : false,
  "enabledChefSupport" : false,
  "rclass" : "local"
}
`

func TestGetMaxUniqueSnapshots(t *testing.T) {
	testCases := []struct {
		packageType                string
		expectedMaxUniqueSnapshots int
	}{
		{conan, -1},
		{maven, 5},
		{gradle, 5},
		{nuget, 5},
		{ivy, 5},
		{sbt, 5},
		{docker, 3},
	}

	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		packageType := strings.TrimSuffix(strings.TrimPrefix(r.RequestURI, "/api/repositories/"), "-local")
		var response string
		if packageType == "docker" {
			response = fmt.Sprintf(repoConfigurationResponse, packageType, 0, 3)
		} else if packageType == "maven" || packageType == "gradle" || packageType == "nuget" || packageType == "ivy" || packageType == "sbt" {
			response = fmt.Sprintf(repoConfigurationResponse, packageType, 5, 0)
		} else {
			assert.Fail(t, "tried to get the Max Unique Snapshots setting of a repository of an unsupported package type")
		}
		_, err := w.Write([]byte(response))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	for _, testCase := range testCases {
		t.Run(testCase.packageType, func(t *testing.T) {
			lowerPackageType := strings.ToLower(testCase.packageType)
			repoSummary := &utils.RepositorySummary{RepoKey: lowerPackageType + "-local", PackageType: testCase.packageType}
			maxUniqueSnapshots, err := getMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedMaxUniqueSnapshots, maxUniqueSnapshots)
		})
	}
}

func TestUpdateMaxUniqueSnapshots(t *testing.T) {
	packageTypes := []string{conan, maven, gradle, nuget, ivy, sbt, docker}

	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		repoDetails := &services.RepositoryDetails{}
		assert.NoError(t, json.Unmarshal(body, repoDetails))
		packageType := repoDetails.PackageType

		expectedPackageType := strings.TrimPrefix(r.RequestURI, "/api/repositories/")
		if strings.HasSuffix(expectedPackageType, "-local") {
			expectedPackageType = strings.TrimSuffix(expectedPackageType, "-local")
			assert.Equal(t, services.LocalRepositoryRepoType, repoDetails.Rclass)
		} else {
			expectedPackageType = strings.TrimSuffix(expectedPackageType, "-federated")
			assert.Equal(t, services.FederatedRepositoryRepoType, repoDetails.Rclass)
		}

		assert.Equal(t, expectedPackageType, packageType)
		if repoDetails.PackageType == "docker" {
			assert.Contains(t, string(body), "\"maxUniqueTags\":5")
		} else if packageType == "maven" || packageType == "gradle" || packageType == "nuget" || packageType == "ivy" || packageType == "sbt" {
			assert.Contains(t, string(body), "\"maxUniqueSnapshots\":5")
		} else {
			assert.Fail(t, "tried to update the Max Unique Snapshots setting of a repository of an unsupported package type")
		}
		_, err = w.Write([]byte(fmt.Sprintf("Repository %s-local update successfully.", packageType)))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	for _, packageType := range packageTypes {
		t.Run(packageType, func(t *testing.T) {
			lowerPackageType := strings.ToLower(packageType)
			repoSummary := &utils.RepositorySummary{RepoKey: lowerPackageType + "-local", PackageType: packageType, RepoType: "LOCAL"}
			err := updateMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary, 5)
			assert.NoError(t, err)

			repoSummary = &utils.RepositorySummary{RepoKey: lowerPackageType + "-federated", PackageType: packageType, RepoType: "FEDERATED"}
			err = updateMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary, 5)
			assert.NoError(t, err)
		})
	}
}

// Create mock server to test transfer config commands
// t           - The testing object
// testHandler - The HTTP handler of the test
func createMockServer(t *testing.T, testHandler transferFilesHandler) (*httptest.Server, *config.ServerDetails, *srcUserPluginService) {
	testServer := httptest.NewServer(http.HandlerFunc(testHandler))
	serverDetails := &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := createSrcRtUserPluginServiceManager(context.Background(), serverDetails)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}
