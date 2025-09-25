package commands

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jfrog/jfrog-cli-core/v2/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func LicenseDeploy(bucketId, jpdId string, flags *LicenseDeployFlags) error {
	postContent := LicenseDeployRequestContent{
		JpdId:        jpdId,
		LicenseCount: flags.LicenseCount,
	}
	requestContent, err := json.Marshal(postContent)
	if err != nil {
		return errorutils.CheckErrorf("Failed to marshal json: %s", err.Error())
	}
	missionControlUrl := flags.ServerDetails.MissionControlUrl + "api/v1/buckets/" + bucketId + "/deploy"
	httpClientDetails := utils.GetMissionControlHttpClientDetails(flags.ServerDetails)
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return err
	}
	resp, body, err := client.SendPost(missionControlUrl, requestContent, httpClientDetails, "")
	if err != nil {
		return err
	}
	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return errors.New(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body))
	}
	log.Debug("Mission Control response: " + resp.Status)
	log.Output(clientutils.IndentJson(body))
	return nil
}

type LicenseDeployRequestContent struct {
	JpdId        string `json:"jpd_id,omitempty"`
	LicenseCount int    `json:"license_count,omitempty"`
}

type LicenseDeployFlags struct {
	ServerDetails *config.ServerDetails
	LicenseCount  int
}
