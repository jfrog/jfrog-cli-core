package commands

import (
	"net/http"

	"github.com/jfrog/jfrog-cli-core/v2/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func JpdAdd(flags *JpdAddFlags) ([]byte, error) {
	missionControlUrl := flags.ServerDetails.MissionControlUrl + "api/v1/jpds"
	httpClientDetails := utils.GetMissionControlHttpClientDetails(flags.ServerDetails)
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return nil, err
	}
	resp, body, err := client.SendPost(missionControlUrl, flags.JpdConfig, httpClientDetails, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, errorutils.CheckErrorf("%s. %s", resp.Status, utils.ReadMissionControlHttpMessage(body))
	}

	log.Debug("Mission Control response: " + resp.Status)
	return body, nil
}

type JpdAddFlags struct {
	ServerDetails *config.ServerDetails
	JpdConfig     []byte
}
