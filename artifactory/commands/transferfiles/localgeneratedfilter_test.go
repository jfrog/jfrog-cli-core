package transferfiles

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	locallyGeneratedEnabled := NewLocallyGenerated(context.Background(), servicesManager, minArtifactoryVersionForLocallyGenerated)
	locallyGeneratedDisabled := NewLocallyGenerated(context.Background(), servicesManager, "7.0.0")

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

func TestFilterLocallyGeneratedMaxRequests(t *testing.T) {
	var parallelRequestsCount int32
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		// Make sure the number of requests in parallel is less than 2
		assert.Less(t, parallelRequestsCount, int32(maxConcurrentLocallyGeneratedRequests))

		// Increment the number of parallel requests
		atomic.AddInt32(&parallelRequestsCount, 1)
		defer atomic.AddInt32(&parallelRequestsCount, -1)

		// Send the response
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("{}"))
		assert.NoError(t, err)
		time.Sleep(time.Millisecond * 10)
	})
	defer testServer.Close()
	locallyGenerated := NewLocallyGenerated(context.Background(), servicesManager, minArtifactoryVersionForLocallyGenerated)

	// Run FilterLocallyGenerated 10 times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			_, err := locallyGenerated.FilterLocallyGenerated([]utils.ResultItem{{Path: "a", Name: "b"}})
			assert.NoError(t, err)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestFilterLocallyGeneratedEnabled(t *testing.T) {
	testServer, _, servicesManager := tests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer testServer.Close()

	assert.True(t, NewLocallyGenerated(context.Background(), servicesManager, minArtifactoryVersionForLocallyGenerated).IsEnabled())
	assert.True(t, NewLocallyGenerated(context.Background(), servicesManager, "8.0.0").IsEnabled())
	assert.False(t, NewLocallyGenerated(context.Background(), servicesManager, "7.54.5").IsEnabled())
	assert.False(t, NewLocallyGenerated(context.Background(), servicesManager, "6.0.0").IsEnabled())
}
