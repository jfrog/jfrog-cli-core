package utils

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"net/http"
)

func GetTransferPluginVersion(client *jfroghttpclient.JfrogHttpClient, url, pluginName string, rtDetails *httputils.HttpClientDetails) (string, error) {
	versionResp, versionBody, _, err := client.SendGet(url, false, rtDetails)
	if err != nil {
		return "", err
	}
	if versionResp.StatusCode == http.StatusOK {
		verRes := &VersionResponse{}
		err = json.Unmarshal(versionBody, verRes)
		if err != nil {
			return "", errorutils.CheckError(err)
		}
		return verRes.Version, nil
	}

	messageFormat := fmt.Sprintf("Response from Artifactory: %s.\n%s\n", versionResp.Status, versionBody)
	if versionResp.StatusCode == http.StatusNotFound {
		return "", errorutils.CheckErrorf("%sIt looks like the %s plugin is not installed on the source server.", messageFormat, pluginName)
	} else {
		// 403 if the user is not admin, 500+ if there is a server error
		return "", errorutils.CheckErrorf(messageFormat)
	}
}

type VersionResponse struct {
	Version string `json:"version,omitempty"`
}
