package transferfiles

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

var filterLocallyGeneratedCases = []struct {
	paths        []utils.ResultItem
	returnedPath []string
}{
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}}, returnedPath: []string{}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}}, returnedPath: []string{"a/b"}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}, {Path: "a/b", Name: "e"}}, returnedPath: []string{"a/b/e"}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}, {Path: "a/b", Name: "e"}}, returnedPath: []string{"a/b", "a/b/e"}},
}

func TestFilterLocallyGenerated(t *testing.T) {
	var err error
	var returnedLocallyGenerated = []byte{}
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+locallyGeneratedApi {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(returnedLocallyGenerated)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	locallyGeneratedEnabled := NewLocallyGenerated(servicesManager, minArtifactoryVersionForLocallyGenerated)
	locallyGeneratedDisabled := NewLocallyGenerated(servicesManager, "7.0.0")

	for _, testCase := range filterLocallyGeneratedCases {
		t.Run("", func(t *testing.T) {
			returnedPayload := &LocallyGeneratedPayload{Paths: testCase.returnedPath}
			returnedLocallyGenerated, err = json.Marshal(returnedPayload)
			assert.NoError(t, err)

			results, err := locallyGeneratedEnabled.FilterLocallyGenerated(testCase.paths)
			assert.NoError(t, err)
			assert.Len(t, results, len(testCase.returnedPath))
			for i := range results {
				assert.Contains(t, testCase.returnedPath, getPathInRepo(&results[i]))
			}

			results, err = locallyGeneratedDisabled.FilterLocallyGenerated(testCase.paths)
			assert.NoError(t, err)
			assert.Equal(t, results, testCase.paths)
		})
	}
}

func TestFilterLocallyGeneratedEnabled(t *testing.T) {
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer testServer.Close()

	assert.True(t, NewLocallyGenerated(servicesManager, minArtifactoryVersionForLocallyGenerated).IsEnabled())
	assert.True(t, NewLocallyGenerated(servicesManager, "8.0.0").IsEnabled())
	assert.False(t, NewLocallyGenerated(servicesManager, "7.54.5").IsEnabled())
	assert.False(t, NewLocallyGenerated(servicesManager, "6.0.0").IsEnabled())
}
