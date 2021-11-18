package commands

import (
	"encoding/json"
	"net/http"

	"github.com/jfrog/jfrog-cli-core/v2/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func LicenseRelease(bucketId, jpdId string, mcDetails *config.ServerDetails) error {
	postContent := LicenseReleaseRequestContent{
		Name: jpdId}
	requestContent, err := json.Marshal(postContent)
	if err != nil {
		return errorutils.CheckErrorf("Failed to marshal json: " + err.Error())
	}
	missionControlUrl := mcDetails.MissionControlUrl + "api/v1/buckets/" + bucketId + "/release"
	httpClientDetails := utils.GetMissionControlHttpClientDetails(mcDetails)
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return err
	}
	resp, body, err := client.SendPost(missionControlUrl, requestContent, httpClientDetails, "")
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return errorutils.CheckErrorf(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body))
	}
	log.Debug("Mission Control response: " + resp.Status)
	return nil
}

type LicenseReleaseRequestContent struct {
	Name string `json:"name,omitempty"`
}
