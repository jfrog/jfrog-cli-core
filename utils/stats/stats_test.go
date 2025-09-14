package coreStats

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"testing"

	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	clientStats "github.com/jfrog/jfrog-client-go/utils/stats"
	"github.com/stretchr/testify/assert"
)

type statsTestFunc func(client *httpclient.HttpClient, artifactoryUrl string, hd httputils.HttpClientDetails) ([]byte, error)

func setupTestClient(t *testing.T) (*httpclient.HttpClient, httputils.HttpClientDetails, string) {
	serverDetails, err := config.GetDefaultServerConf()
	assert.NoError(t, err)

	httpClientDetails := httputils.HttpClientDetails{AccessToken: serverDetails.AccessToken}
	httpClientDetails.SetContentTypeApplicationJson()
	client, err := httpclient.ClientBuilder().Build()
	assert.NoError(t, err)
	return client, httpClientDetails, serverDetails.GetUrl()
}

func TestAllProductAPIs(t *testing.T) {
	productFunctions := map[string]statsTestFunc{
		"Artifactory":    clientStats.GetArtifactoryStats,
		"Repositories":   clientStats.GetRepositoriesStats,
		"XrayPolicies":   clientStats.GetXrayPolicies,
		"XrayWatches":    clientStats.GetXrayWatches,
		"Projects":       clientStats.GetProjectsStats,
		"JPDs":           clientStats.GetJPDsStats,
		"ReleaseBundles": clientStats.GetReleaseBundlesStats,
	}

	for product, getFunc := range productFunctions {
		t.Run(product, func(t *testing.T) {
			t.Run(product, func(t *testing.T) {
				client, httpClientDetails, baseUrl := setupTestClient(t)
				_, err := getFunc(client, baseUrl, httpClientDetails)
				if err != nil {
					assert.NoError(t, err)
				}
			})
		})
	}
}
