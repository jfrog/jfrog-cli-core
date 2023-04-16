package transferfiles

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

var filterLocalGeneratedCases = []struct {
	paths        []utils.ResultItem
	returnedPath []string
}{
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}}, returnedPath: []string{}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}}, returnedPath: []string{"a/b"}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}, {Path: "a/b", Name: "e"}}, returnedPath: []string{"a/b/e"}},
	{paths: []utils.ResultItem{{Path: "a", Name: "b"}, {Path: "a/b", Name: "e"}}, returnedPath: []string{"a/b", "a/b/e"}},
}

func TestFilterLocalGenerated(t *testing.T) {
	var err error
	var returnedLocalGenerated = []byte{}
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+localGeneratedApi {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(returnedLocalGenerated)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	localGeneratedEnabled := NewLocalGenerated(servicesManager, minArtifactoryVersionForLocalGenerated)
	localGeneratedDisabled := NewLocalGenerated(servicesManager, "7.0.0")

	for _, testCase := range filterLocalGeneratedCases {
		t.Run("", func(t *testing.T) {
			returnedPayload := &LocalGeneratedPayload{Paths: testCase.returnedPath}
			returnedLocalGenerated, err = json.Marshal(returnedPayload)
			assert.NoError(t, err)

			results, err := localGeneratedEnabled.FilterLocalGenerated(testCase.paths)
			assert.NoError(t, err)
			assert.Len(t, results, len(testCase.returnedPath))
			for i := range results {
				assert.Contains(t, testCase.returnedPath, getPathInRepo(&results[i]))
			}

			results, err = localGeneratedDisabled.FilterLocalGenerated(testCase.paths)
			assert.NoError(t, err)
			assert.Equal(t, results, testCase.paths)
		})
	}
}

func TestFilterLocalGeneratedEnabled(t *testing.T) {
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer testServer.Close()

	assert.True(t, NewLocalGenerated(servicesManager, minArtifactoryVersionForLocalGenerated).IsEnabled())
	assert.True(t, NewLocalGenerated(servicesManager, "8.0.0").IsEnabled())
	assert.False(t, NewLocalGenerated(servicesManager, "7.54.5").IsEnabled())
	assert.False(t, NewLocalGenerated(servicesManager, "6.0.0").IsEnabled())
}
