package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"net/http"
)

const (
	syncChunks  = "syncChunks"
	uploadChunk = "uploadChunk"
)

type VerifyCompatibilityResponse struct {
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

type srcUserPluginService struct {
	client     *jfroghttpclient.JfrogHttpClient
	artDetails *auth.ServiceDetails
}

func NewSrcUserPluginService(artDetails auth.ServiceDetails, client *jfroghttpclient.JfrogHttpClient) *srcUserPluginService {
	return &srcUserPluginService{artDetails: &artDetails, client: client}
}

func (sup *srcUserPluginService) GetArtifactoryDetails() auth.ServiceDetails {
	return *sup.artDetails
}

func (sup *srcUserPluginService) GetJfrogHttpClient() *jfroghttpclient.JfrogHttpClient {
	return sup.client
}

func (sup *srcUserPluginService) IsDryRun() bool {
	return false
}

func (sup *srcUserPluginService) syncChunks(ucStatus api.UploadChunksStatusBody) (api.UploadChunksStatusResponse, error) {
	content, err := json.Marshal(ucStatus)
	if err != nil {
		return api.UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	httpDetails.SetContentTypeApplicationJson()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+commandsUtils.PluginsExecuteRestApi+syncChunks, content, &httpDetails)
	if err != nil {
		return api.UploadChunksStatusResponse{}, err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return api.UploadChunksStatusResponse{}, err
	}

	var statusResponse api.UploadChunksStatusResponse
	err = json.Unmarshal(body, &statusResponse)
	if err != nil {
		return api.UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}
	return statusResponse, nil
}

// Uploads a chunk of files.
// If no error occurred, returns an uuid token to get chunk status with.
func (sup *srcUserPluginService) uploadChunk(chunk api.UploadChunk) (uploadChunkResponse api.UploadChunkResponse, err error) {
	content, err := json.Marshal(chunk)
	if err != nil {
		return api.UploadChunkResponse{}, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	httpDetails.SetContentTypeApplicationJson()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+commandsUtils.PluginsExecuteRestApi+uploadChunk, content, &httpDetails)
	if err != nil {
		return api.UploadChunkResponse{}, err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusAccepted); err != nil {
		return api.UploadChunkResponse{}, err
	}

	var uploadResponse api.UploadChunkResponse
	err = json.Unmarshal(body, &uploadResponse)
	if err != nil {
		return api.UploadChunkResponse{}, errorutils.CheckError(err)
	}
	if uploadResponse.UuidToken == "" {
		return api.UploadChunkResponse{}, errorutils.CheckErrorf("unexpected empty token returned for chunk upload")
	}
	if uploadResponse.NodeId == "" {
		return api.UploadChunkResponse{}, errorutils.CheckErrorf("unexpected empty node id returned for chunk upload")
	}
	return uploadResponse, nil
}

func (sup *srcUserPluginService) version() (string, error) {
	dataTransferVersionUrl := sup.GetArtifactoryDetails().GetUrl() + commandsUtils.PluginsExecuteRestApi + "dataTransferVersion"
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	return commandsUtils.GetTransferPluginVersion(sup.client, dataTransferVersionUrl, "data-transfer", commandsUtils.Source, &httpDetails)
}

func (sup *srcUserPluginService) verifyCompatibilityRequest() (*VerifyCompatibilityResponse, error) {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	httpDetails.SetContentTypeApplicationJson()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+commandsUtils.PluginsExecuteRestApi+"verifyCompatibility", []byte("{}"), &httpDetails)
	if err != nil {
		return nil, err
	}

	err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var result VerifyCompatibilityResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	return &result, nil
}

func (sup *srcUserPluginService) verifyConnectivityRequest(targetAuth api.TargetAuth) error {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	httpDetails.SetContentTypeApplicationJson()
	content, err := json.Marshal(targetAuth)
	if err != nil {
		return errorutils.CheckError(err)
	}
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+commandsUtils.PluginsExecuteRestApi+"verifySourceTargetConnectivity", content, &httpDetails)
	if err != nil {
		return err
	}

	return errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK)
}

func (sup *srcUserPluginService) stop() (nodeId string, err error) {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+commandsUtils.PluginsExecuteRestApi+"stop", []byte{}, &httpDetails)
	if err != nil {
		return "", err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return "", err
	}

	var result api.NodeIdResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return result.NodeId, nil
}
