package utils

import (
	"encoding/json"
	"io/ioutil"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// Download and unmarshal a file from artifactory.
func RemoteUnmarshal(serviceManager artifactory.ArtifactoryServicesManager, url string, loadTarget interface{}) (err error) {
	ioReaderCloser, err := serviceManager.ReadRemoteFile(url)
	if err != nil {
		return
	}
	defer func() {
		if localErr := ioReaderCloser.Close(); err == nil {
			err = localErr
		}
	}()
	content, err := ioutil.ReadAll(ioReaderCloser)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(json.Unmarshal(content, loadTarget))
}
