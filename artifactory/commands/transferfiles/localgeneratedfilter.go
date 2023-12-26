package transferfiles

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/sync/semaphore"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	locallyGeneratedApi                      = "api/localgenerated/filter/paths"
	minArtifactoryVersionForLocallyGenerated = "7.55.0"
	maxConcurrentLocallyGeneratedRequests    = 5
)

// The request and response payload of POST '/api/localgenerated/filter/paths'
type LocallyGeneratedPayload struct {
	RepoKey string   `json:"repoKey,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

type locallyGeneratedFilter struct {
	httpDetails          *httputils.HttpClientDetails
	targetServiceDetails auth.ServiceDetails
	context              context.Context
	// Semaphore to limit the max concurrent Locally Generated requests to 2
	sem     semaphore.Weighted
	enabled bool
}

func NewLocallyGenerated(context context.Context, serviceManager artifactory.ArtifactoryServicesManager, artifactoryVersion string) *locallyGeneratedFilter {
	serviceDetails := serviceManager.GetConfig().GetServiceDetails()
	httpDetails := serviceDetails.CreateHttpClientDetails()
	utils.SetContentType("application/json", &httpDetails.Headers)

	enabled := version.NewVersion(artifactoryVersion).AtLeast(minArtifactoryVersionForLocallyGenerated)
	log.Debug("Locally generated filter enabled:", enabled)
	return &locallyGeneratedFilter{
		enabled:              enabled,
		targetServiceDetails: serviceDetails,
		httpDetails:          &httpDetails,
		context:              context,
		sem:                  *semaphore.NewWeighted(maxConcurrentLocallyGeneratedRequests),
	}
}

// Filters out locally generated files.
// Files that are generated automatically by Artifactory on the target instance (also known as "locally generated files") should not be transferred.
// aqlResults - Directory content in phase 1 or 15 minutes interval results in phase 2.
func (lg *locallyGeneratedFilter) FilterLocallyGenerated(aqlResultItems []utils.ResultItem) ([]utils.ResultItem, error) {
	if !lg.enabled || len(aqlResultItems) == 0 {
		return aqlResultItems, nil
	}
	content, err := lg.createPayload(aqlResultItems)
	if err != nil || len(content) == 0 {
		return []utils.ResultItem{}, err
	}

	resp, body, err := lg.doFilterLocallyGenerated(content)
	if err != nil {
		return []utils.ResultItem{}, err
	}

	nonLocallyGeneratedPaths, err := lg.parseResponse(resp, body)
	if err != nil {
		return []utils.ResultItem{}, err
	}

	return lg.getNonLocallyGeneratedResults(aqlResultItems, nonLocallyGeneratedPaths), err
}

// Send 'POST /localgenerated/filter/paths' request under Semaphore restriction to prevent more than 2 requests in parallel.
// We limit the number of request by 2 to prevent excessive load on the target server.
// content - The rest API body which is a byte array of LocallyGeneratedPayload.
func (lg *locallyGeneratedFilter) doFilterLocallyGenerated(content []byte) (resp *http.Response, body []byte, err error) {
	if err := lg.sem.Acquire(lg.context, 1); err != nil {
		return nil, []byte{}, errorutils.CheckError(err)
	}
	defer lg.sem.Release(1)
	return lg.targetServiceDetails.GetClient().SendPost(lg.targetServiceDetails.GetUrl()+locallyGeneratedApi, content, lg.httpDetails)
}

// Return true if should filter Artifactory locally generated files in the JFrog CLI
// False if should filter Artifactory locally generated files in the Data Transfer plugin
func (lg *locallyGeneratedFilter) IsEnabled() bool {
	return lg.enabled
}

// Create payload for the POST '/api/localgenerated/filter/paths' REST API
// aqlResultItems - Directory content in phase 1 or 15 minutes interval results in phase 2
func (lg *locallyGeneratedFilter) createPayload(aqlResultItems []utils.ResultItem) ([]byte, error) {
	payload := &LocallyGeneratedPayload{
		RepoKey: aqlResultItems[0].Repo,
		Paths:   make([]string, 0, len(aqlResultItems)),
	}
	for i, aqlResultItem := range aqlResultItems {
		// Filter out the root folder in the repository
		if aqlResultItem.Repo != "." || aqlResultItem.Name != "." {
			payload.Paths = append(payload.Paths, getPathInRepo(&aqlResultItems[i]))
		}
	}
	if len(payload.Paths) == 0 {
		return []byte{}, nil
	}

	content, err := json.Marshal(payload)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return content, nil
}

// Parse the response from Artifactory for the POST '/api/localgenerated/filter/paths'
// resp - Response status from Artifactory
// body - Response body from Artifactory
// Return a set of non locally generated paths - the files and directories to transfer.
func (lg *locallyGeneratedFilter) parseResponse(resp *http.Response, body []byte) (*datastructures.Set[string], error) {
	if err := errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return nil, err
	}

	response := &LocallyGeneratedPayload{}
	err := json.Unmarshal(body, response)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	nonLocallyGeneratedPaths := datastructures.MakeSet[string]()
	for _, path := range response.Paths {
		nonLocallyGeneratedPaths.Add(path)
	}

	return nonLocallyGeneratedPaths, errorutils.CheckError(err)
}

// Get the non-locally-generated AQL results.
// aqlResultItems - Directory content in phase 1 or 15 minutes interval results in phase 2
// nonLocallyGeneratedPaths - Non locally generated paths
func (lg *locallyGeneratedFilter) getNonLocallyGeneratedResults(aqlResultItems []utils.ResultItem, nonLocallyGeneratedPaths *datastructures.Set[string]) (nonLocallyGeneratedAqlResults []utils.ResultItem) {
	nonLocallyGeneratedAqlResults = make([]utils.ResultItem, 0, nonLocallyGeneratedPaths.Size())
	for i := range aqlResultItems {
		pathInRepo := getPathInRepo(&aqlResultItems[i])
		if nonLocallyGeneratedPaths.Exists(pathInRepo) {
			nonLocallyGeneratedAqlResults = append(nonLocallyGeneratedAqlResults, aqlResultItems[i])
		} else {
			log.Debug("Excluding locally generated item from being transferred:", pathInRepo)
		}
	}
	return nonLocallyGeneratedAqlResults
}

func getPathInRepo(aqlResultItem *utils.ResultItem) string {
	if aqlResultItem.Path == "." {
		return aqlResultItem.Name
	}
	return aqlResultItem.Path + "/" + aqlResultItem.Name
}
