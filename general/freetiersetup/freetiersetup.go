package freetiersetup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/pkg/browser"
)

const (
	myJfrogEndPoint          = "https://myjfrog-api.jfrog.info/api/v1/activation/cloud/cli/getStatus/"
	defaultSyncSleepInterval = 5 * time.Second  // 5 seconds
	maxWaitMinutes           = 10 * time.Minute // 10 minutes
)

type FreeTierSetupCommand struct {
	registrationURL string
	id              uuid.UUID
	serverDetails   *config.ServerDetails
	progress        ioUtils.ProgressMgr
}

func (ftc *FreeTierSetupCommand) ServerDetails() (*config.ServerDetails, error) {
	return ftc.serverDetails, nil
}

func (ftc *FreeTierSetupCommand) SetProgress(progress ioUtils.ProgressMgr) {
	ftc.progress = progress
}

func (ftc *FreeTierSetupCommand) IsFileProgress() bool {
	return false
}

func NewFreeTierSetupCommand(url string) *FreeTierSetupCommand {
	return &FreeTierSetupCommand{
		registrationURL: url,
		id:              uuid.New(),
	}
}

func (ftc *FreeTierSetupCommand) Run() (err error) {
	browser.OpenURL(ftc.registrationURL + "?id=" + ftc.id.String())
	ftc.progress.SetHeadlineMsg("Welcome to JFrog CLI! To complete your JFrog environment setup, please fill out the details in your browser")
	server, err := ftc.getServerDetails()
	if err != nil {
		return
	}
	err = configServer(server)
	if err != nil {
		return err
	}
	println("Your new JFrog environment is ready!")
	println("	1.CD into your code project directory")
	println("	2.Run \"jf project init\"")
	println("	3.Read more about how to get started at https://...")
	return nil
}

func (ftc *FreeTierSetupCommand) CommandName() string {
	return "setup"
}

func (ftc *FreeTierSetupCommand) getServerDetails() (serverDetails *config.ServerDetails, err error) {
	requestBody := &myJfrogGetStatusRequest{CliRegistrationId: ftc.id.String()}
	requestContent, err := json.Marshal(requestBody)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}

	httpClientDetails := httputils.HttpClientDetails{
		Headers: map[string]string{"Content-Type": "application/json"},
	}
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return nil, err
	}
	message := fmt.Sprintf("Sync: Get MyJFrog status report. Request ID:%s...", ftc.id)

	pollingAction := func() (shouldStop bool, responseBody []byte, err error) {
		log.Debug(message)
		resp, body, err := client.SendPost(myJfrogEndPoint, requestContent, httpClientDetails, "")
		if err != nil {
			return true, nil, err
		}
		if err = errorutils.CheckResponseStatus(resp, http.StatusOK, http.StatusNotFound); err != nil {
			err = errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
			return true, nil, err
		}
		log.Debug(message)
		// Got the a valid response. waits for ready=true
		if resp.StatusCode == http.StatusOK {
			ftc.progress.SetHeadlineMsg("Ready for your DevOps journey? Please hang on while we creat your environment")
			statusResponse := myJfrogGetStatusResponse{}
			if err = json.Unmarshal(body, &statusResponse); err != nil {
				return true, nil, err
			}
			// Got the new server deatiles
			if statusResponse.Ready {
				return true, body, nil
			}
		}
		return false, nil, nil
	}

	pollingExecutor := &httputils.PollingExecutor{
		Timeout:         maxWaitMinutes,
		PollingInterval: defaultSyncSleepInterval,
		PollingAction:   pollingAction,
	}

	body, err := pollingExecutor.Execute()
	if err != nil {
		return nil, err
	}
	statusResponse := myJfrogGetStatusResponse{}
	if err = json.Unmarshal(body, &statusResponse); err != nil {
		return nil, errorutils.CheckError(err)
	}
	ftc.progress.ClearHeadlineMsg()
	serverDetails = &config.ServerDetails{
		Url:         statusResponse.PlatformUrl,
		AccessToken: statusResponse.AccessToken,
	}
	ftc.serverDetails = serverDetails
	return serverDetails, nil
}

func configServer(server *config.ServerDetails) error {
	u, err := url.Parse(server.Url)
	if errorutils.CheckError(err) != nil {
		return err
	}
	// Take the server name from host name: https://myjfrog.jfrog.com/ -> myjfrog
	serverId := strings.Split(u.Host, ".")[0]
	configCmd := commands.NewConfigCommand().SetInteractive(false).SetServerId(serverId).SetDetails(server)
	return configCmd.Config()

}

type myJfrogGetStatusRequest struct {
	CliRegistrationId string `json:"cliRegistrationId,omitempty"`
}

type myJfrogGetStatusResponse struct {
	CliRegistrationId string `json:"cliRegistrationId,omitempty"`
	Ready             bool   `json:"ready,omitempty"`
	AccessToken       string `json:"accessToken,omitempty"`
	PlatformUrl       string `json:"platformUrl,omitempty"`
}
