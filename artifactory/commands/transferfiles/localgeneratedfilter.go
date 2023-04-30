package transferfiles

import (
	"encoding/json"
	"net/http"

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
)

// The request and response payload of POST '/api/localgenerated/filter/paths'
type LocallyGeneratedPayload struct {
	RepoKey string   `json:"repoKey,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

type LocallyGeneratedFilter struct {
	httpDetails          *httputils.HttpClientDetails
	targetServiceDetails auth.ServiceDetails
	enabled              bool
}

func NewLocallyGenerated(serviceManager artifactory.ArtifactoryServicesManager, artifactoryVersion string) *LocallyGeneratedFilter {
	serviceDetails := serviceManager.GetConfig().GetServiceDetails()
	httpDetails := serviceDetails.CreateHttpClientDetails()
	utils.SetContentType("application/json", &httpDetails.Headers)

	enabled := version.NewVersion(artifactoryVersion).AtLeast(minArtifactoryVersionForLocallyGenerated)
	log.Debug("Locally generated filter enabled:", enabled)
	return &LocallyGeneratedFilter{
		enabled:              enabled,
		targetServiceDetails: serviceDetails,
		httpDetails:          &httpDetails,
	}
}

// Filters out locally generated files.
// Files that are generated automatically by Artifactory on the target instance (also known as "locally generated files") should not be transferred.
// aqlResults - Directory content in phase 1 or 15 minutes interval results in phase 2.
func (lg *LocallyGeneratedFilter) FilterLocallyGenerated(aqlResultItems []utils.ResultItem) ([]utils.ResultItem, error) {
	if !lg.enabled || len(aqlResultItems) == 0 {
		return aqlResultItems, nil
	}
	content, err := lg.createPayload(aqlResultItems)
	if err != nil {
		return []utils.ResultItem{}, err
	}

	resp, body, err := lg.targetServiceDetails.GetClient().SendPost(lg.targetServiceDetails.GetUrl()+locallyGeneratedApi, content, lg.httpDetails)
	if err != nil {
		return []utils.ResultItem{}, err
	}

	nonLocallyGeneratedPaths, err := lg.parseResponse(resp, body)
	if err != nil {
		return []utils.ResultItem{}, err
	}

	return lg.getNonLocallyGeneratedResults(aqlResultItems, nonLocallyGeneratedPaths), err
}

// Return true if should filter Artifactory locally generated files in the JFrog CLI
// False if should filter Artifactory locally generated files in the Data Transfer plugin
func (lg *LocallyGeneratedFilter) IsEnabled() bool {
	return lg.enabled
}

// Create payload for the POST '/api/localgenerated/filter/paths' REST API
// aqlResultItems - Directory content in phase 1 or 15 minutes interval results in phase 2
func (lg *LocallyGeneratedFilter) createPayload(aqlResultItems []utils.ResultItem) ([]byte, error) {
	payload := &LocallyGeneratedPayload{
		RepoKey: aqlResultItems[0].Repo,
		Paths:   make([]string, 0, len(aqlResultItems)),
	}
	for i := range aqlResultItems {
		payload.Paths = append(payload.Paths, getPathInRepo(&aqlResultItems[i]))
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
func (lg *LocallyGeneratedFilter) parseResponse(resp *http.Response, body []byte) (*datastructures.Set[string], error) {
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
func (lg *LocallyGeneratedFilter) getNonLocallyGeneratedResults(aqlResultItems []utils.ResultItem, nonLocallyGeneratedPaths *datastructures.Set[string]) (nonLocallyGeneratedAqlResults []utils.ResultItem) {
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
	return aqlResultItem.Path + "/" + aqlResultItem.Name
}
