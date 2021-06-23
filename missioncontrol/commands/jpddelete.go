package commands

import (
	"errors"

	"github.com/jfrog/jfrog-cli-core/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
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
		return errorutils.CheckError(errors.New(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body)))
	}
	log.Debug("Mission Control response: " + resp.Status)
	return nil
}
