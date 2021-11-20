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

func LicenseAcquire(bucketId string, name string, serverDetails *config.ServerDetails) error {
	postContent := LicenseAcquireRequestContent{
		Name:         name,
		LicenseCount: 1,
	}
	requestContent, err := json.Marshal(postContent)
	if err != nil {
		return errorutils.CheckErrorf("Failed to marshal json: " + err.Error())
	}
	missionControlUrl := serverDetails.MissionControlUrl + "api/v1/buckets/" + bucketId + "/acquire"
	httpClientDetails := utils.GetMissionControlHttpClientDetails(serverDetails)
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return err
	}
	resp, body, err := client.SendPost(missionControlUrl, requestContent, httpClientDetails, "")
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errorutils.CheckErrorf(resp.Status + ". " + utils.ReadMissionControlHttpMessage(body))
	}
	log.Debug("Mission Control response: " + resp.Status)

	// Extract license from response
	var licenseKeys licenseKeys
	err = json.Unmarshal(body, &licenseKeys)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if len(licenseKeys.LicenseKeys) < 1 {
		return errorutils.CheckErrorf("failed to acquire license key from Mission Control: received 0 license keys")
	}
	// Print license to log
	log.Output(licenseKeys.LicenseKeys[0])
	return nil
}

type LicenseAcquireRequestContent struct {
	Name         string `json:"name,omitempty"`
	LicenseCount int    `json:"license_count,omitempty"`
}

type licenseKeys struct {
	LicenseKeys []string `json:"license_keys,omitempty"`
}
