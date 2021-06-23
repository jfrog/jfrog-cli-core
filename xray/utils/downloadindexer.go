package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"github.com/jfrog/jfrog-client-go/xray"
)

const indexerFileName = "indexer-app"

// TODO: Should be changed back to 3.28.0 before merge
const graphScanMinVersion = "3.0.0"

func DownloadIndexerIfNeeded(xrayManager *xray.XrayServicesManager) (string, error) {
	xrayVersionStr, err := xrayManager.GetVersion()
	if err != nil {
		return "", err
	}
	xrayVersion := version.NewVersion(xrayVersionStr)
	if !xrayVersion.AtLeast(graphScanMinVersion) {
		return "", errorutils.CheckError(errors.New("this operation requires Xray version " + graphScanMinVersion + " or higher"))
	}

	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	downloadDirPath := filepath.Join(dependenciesPath, "xray-indexer", xrayVersionStr)
	indexerPath := filepath.Join(downloadDirPath, indexerFileName)
	exists, err := fileutils.IsFileExists(indexerPath, false)
	if err != nil {
		return "", err
	}
	if exists {
		return indexerPath, nil
	}

	log.Info("JFrog Xray Indexer is not cached locally. Downloading it now...")
	err = downloadIndexer(xrayManager, downloadDirPath)
	if err != nil {
		return "", err
	}
	// Add execution premissions to the indexer
	err = os.Chmod(indexerPath, 0777)
	if err != nil {
		return "", err
	}
	return indexerPath, nil
}

func downloadIndexer(xrayManager *xray.XrayServicesManager, downloadDirPath string) error {
	url := fmt.Sprintf("%sapi/v1/indexer-resources/download/%s/%s", xrayManager.Config().GetServiceDetails().GetUrl(), runtime.GOOS, runtime.GOARCH)
	downloadFileDetails := &httpclient.DownloadFileDetails{
		DownloadPath:  url,
		LocalPath:     downloadDirPath,
		LocalFileName: indexerFileName,
	}
	httpClientDetails := xrayManager.Config().GetServiceDetails().CreateHttpClientDetails()

	resp, err := xrayManager.Client().DownloadFile(downloadFileDetails, "", &httpClientDetails, false)
	if err == nil && resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errorutils.CheckError(errors.New(fmt.Sprintf("%s received when attempting to download %s. An error occurred while trying to read the body of the response: %s", resp.Status, url, err.Error())))
		}
		resp.Body.Close()
		err = errorutils.CheckError(errors.New(fmt.Sprintf("%s received when attempting to download %s\n%s", resp.Status, url, body)))
	}

	return err
}
