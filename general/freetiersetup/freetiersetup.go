package freetiersetup

import (
	"encoding/json"
	"errors"
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
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"

	"github.com/pkg/browser"
)

const (
	myJfrogEndPoint          = "https://myjfrog-api.jfrog.info/api/v1/activation/cloud/cli/getStatus/"
	defaultSyncSleepInterval = 5 * time.Second  // 5 seconds
	maxWaitMinutes           = 30 * time.Minute // 30 minutes
)

type FreeTierSetupCommand struct {
	registrationURL string
	id              uuid.UUID
}

func NewFreeTierSetupCommand(url string) *FreeTierSetupCommand {
	return &FreeTierSetupCommand{url, uuid.New()}
}

func (ftc *FreeTierSetupCommand) Run() (err error) {
	browser.OpenURL(ftc.registrationURL + "?" + ftc.id.String())
	println("We are ready to set up your JFrog Environment.\nPlease fill out the environment details in your browser while keeping this terminal window open.")
	server, err := ftc.getServerDetails()
	if err != nil {
		return
	}
	err = configServer(server)
	return err
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
	message := fmt.Sprintf("## Get MyJFrog status report. Request ID:%s...", ftc.id)
	//
	ticker := time.NewTicker(defaultSyncSleepInterval)
	timeout := make(chan bool)
	errChan := make(chan error)
	resultChan := make(chan []byte)
	endPoint := myJfrogEndPoint
	go func() {
		for {
			select {
			case <-timeout:
				errChan <- errorutils.CheckError(errors.New("Timeout."))
				resultChan <- nil
				return
			case _ = <-ticker.C:
				resp, body, err := client.SendPost(endPoint, requestContent, httpClientDetails, "")
				if err != nil {
					errChan <- err
					resultChan <- nil
					return
				}
				if err = errorutils.CheckResponseStatus(resp, http.StatusOK, http.StatusNotFound); err != nil {
					errChan <- errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
					resultChan <- nil
					return
				}
				fmt.Println(message + " " + fmt.Sprint(resp.StatusCode))
				if resp.StatusCode == http.StatusOK {
					println("Setting up your JFrog Environment")
					statusResponse := myJfrogGetStatusResponse{}
					if err = json.Unmarshal(body, &statusResponse); err != nil {
						errChan <- errorutils.CheckError(err)
						resultChan <- nil
						return
					}
					// Got the ne server deatiles
					if statusResponse.Ready {
						errChan <- nil
						resultChan <- body
						return
					}
				}
			}
		}
	}()
	// Make sure we don't wait forever
	go func() {
		time.Sleep(maxWaitMinutes)
		timeout <- true
	}()
	// Wait for result or error
	err = <-errChan
	body := <-resultChan
	ticker.Stop()
	if err != nil {
		return nil, err
	}
	statusResponse := myJfrogGetStatusResponse{}
	if err = json.Unmarshal(body, &statusResponse); err != nil {
		return nil, errorutils.CheckError(err)
	}
	println("Your JFrog Environment is ready.")
	serverDetails = &config.ServerDetails{
		Url:         statusResponse.PlatformUrl,
		AccessToken: statusResponse.AccessToken,
	}
	return serverDetails, nil
}

func configServer(server *config.ServerDetails) error {
	u, err := url.Parse(server.Url)
	if errorutils.CheckError(err) != nil {
		return err
	}
	// Take the server name from host name: https://myjfrog-api.jfrog.com/ -> myjfrog-api
	serverId := strings.Split(u.Host, ".")[0]
	configCmd := commands.NewConfigCommand().SetInteractive(false).SetServerId(serverId).SetDetails(server)
	return configCmd.Config()

}

func getServerName()

type myJfrogGetStatusRequest struct {
	CliRegistrationId string `json:"cliRegistrationId,omitempty"`
}

type myJfrogGetStatusResponse struct {
	CliRegistrationId string `json:"cliRegistrationId,omitempty"`
	Ready             bool   `json:"ready,omitempty"`
	AccessToken       string `json:"accessToken,omitempty"`
	PlatformUrl       string `json:"platformUrl,omitempty"`
}
