package transferfiles

import (
	"encoding/json"
	"net/http"

	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const pluginsExecuteRestApi = "api/plugins/execute/"
const syncChunks = "syncChunks"

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

func (sup *srcUserPluginService) syncChunks(ucStatus UploadChunksStatusBody) (UploadChunksStatusResponse, error) {
	content, err := json.Marshal(ucStatus)
	if err != nil {
		return UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	utils.SetContentType("application/json", &httpDetails.Headers)
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+pluginsExecuteRestApi+syncChunks, content, &httpDetails)
	if err != nil {
		return UploadChunksStatusResponse{}, err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return UploadChunksStatusResponse{}, err
	}

	var statusResponse UploadChunksStatusResponse
	err = json.Unmarshal(body, &statusResponse)
	if err != nil {
		return UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}
	return statusResponse, nil
}

// Uploads a chunk of files.
// If no error occurred, returns an uuid token to get chunk status with.
func (sup *srcUserPluginService) uploadChunk(chunk UploadChunk) (uuidToken string, err error) {
	content, err := json.Marshal(chunk)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	utils.SetContentType("application/json", &httpDetails.Headers)
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+pluginsExecuteRestApi+"uploadChunk", content, &httpDetails)
	if err != nil {
		return "", err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusAccepted); err != nil {
		return "", err
	}

	var uploadResponse UploadChunkResponse
	err = json.Unmarshal(body, &uploadResponse)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	if uploadResponse.UuidToken == "" {
		return "", errorutils.CheckErrorf("unexpected empty token returned for chunk upload")
	}
	return uploadResponse.UuidToken, nil
}

func (sup *srcUserPluginService) storeProperties(repoKey string) error {
	params := map[string]string{"repoKey": repoKey}
	requestFullUrl, err := utils.BuildArtifactoryUrl(sup.GetArtifactoryDetails().GetUrl(), pluginsExecuteRestApi+"storeProperties", params)
	if err != nil {
		return err
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(requestFullUrl, nil, &httpDetails)
	if err != nil {
		return err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return err
	}

	return nil
}

func (sup *srcUserPluginService) handlePropertiesDiff(requestBody HandlePropertiesDiff) (*HandlePropertiesDiffResponse, error) {
	content, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	utils.SetContentType("application/json", &httpDetails.Headers)
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+pluginsExecuteRestApi+"handlePropertiesDiff", content, &httpDetails)
	if err != nil {
		return nil, err
	}

	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return nil, err
	}

	var result HandlePropertiesDiffResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &result, nil
}

func (sup *srcUserPluginService) version() (string, error) {
	dataTransferVersionUrl := sup.GetArtifactoryDetails().GetUrl() + pluginsExecuteRestApi + "dataTransferVersion"
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	return commandsUtils.GetTransferPluginVersion(sup.client, dataTransferVersionUrl, "data-transfer", commandsUtils.Source, &httpDetails)
}

func (sup *srcUserPluginService) verifyCompatibilityRequest() (*VerifyCompatibilityResponse, error) {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+pluginsExecuteRestApi+"verifyCompatibility", []byte("{}"), &httpDetails)
	if err != nil {
		return nil, err
	}
	var result VerifyCompatibilityResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	err = errorutils.CheckResponseStatus(resp, http.StatusOK)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (sup *srcUserPluginService) stop() (nodeId string, err error) {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+pluginsExecuteRestApi+"stop", []byte{}, &httpDetails)
	if err != nil {
		return "", err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return "", err
	}

	var result NodeIdResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return result.NodeId, nil
}
