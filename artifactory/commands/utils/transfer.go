package utils

import (
	"encoding/json"
	"fmt"
	"github.com/gocarina/gocsv"
	ioutils "github.com/jfrog/gofrog/io"
	logutils "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"net/http"
	"time"
)

type ServerType string

const (
	Source                ServerType = "source"
	Target                ServerType = "target"
	PluginsExecuteRestApi            = "api/plugins/execute/"
)

func GetTransferPluginVersion(client *jfroghttpclient.JfrogHttpClient, url, pluginName string, serverType ServerType, rtDetails *httputils.HttpClientDetails) (string, error) {
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
		return "", errorutils.CheckErrorf("%sIt looks like the %s plugin is not installed on the %s server.", messageFormat, pluginName, serverType)
	} else {
		// 403 if the user is not admin, 500+ if there is a server error
		return "", errorutils.CheckErrorf(messageFormat)
	}
}

type VersionResponse struct {
	Version string `json:"version,omitempty"`
}

func CreateCSVFile(filePrefix string, items interface{}, timeStarted time.Time) (csvPath string, err error) {
	// Create CSV file
	summaryCsv, err := logutils.CreateCustomLogFile(fmt.Sprintf("%s-%s.csv", filePrefix, timeStarted.Format(logutils.DefaultLogTimeLayout)))
	if err != nil {
		return
	}
	csvPath = summaryCsv.Name()
	defer ioutils.Close(summaryCsv, &err)
	// Marshal JSON typed items array to CSV file
	err = errorutils.CheckError(gocsv.MarshalFile(items, summaryCsv))
	return
}
