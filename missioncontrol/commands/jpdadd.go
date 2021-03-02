package commands

import (
	"errors"
	"net/http"

	"github.com/jfrog/jfrog-cli-core/missioncontrol/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func JpdAdd(flags *JpdAddFlags) error {
	missionControlUrl := flags.ServerDetails.MissionControlUrl + "api/v1/jpds"
	httpClientDetails := utils.GetMissionControlHttpClientDetails(flags.ServerDetails)
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return err
	}
	resp, body, err := client.SendPost(missionControlUrl, flags.JpdConfig, httpClientDetails)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return errorutils.CheckError(errors.New(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body)))
	}

	log.Debug("Mission Control response: " + resp.Status)
	log.Output(clientutils.IndentJson(body))
	return nil
}

type JpdAddFlags struct {
	ServerDetails *config.ServerDetails
	JpdConfig     []byte
}
