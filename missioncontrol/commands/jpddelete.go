package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func JpdDelete(jpdId string, serverDetails *config.ServerDetails) error {
	missionControlUrl := serverDetails.MissionControlUrl + "api/v1/jpds/" + jpdId
	httpClientDetails := utils.GetMissionControlHttpClientDetails(serverDetails)
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return err
	}
	resp, body, err := client.SendDelete(missionControlUrl, nil, httpClientDetails, "")
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return errorutils.CheckErrorf(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body))
	}
	log.Debug("Mission Control response: " + resp.Status)
	return nil
}
