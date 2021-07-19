package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"github.com/jfrog/jfrog-client-go/xray"
)

const (
	indexerFileName     = "indexer-app"
	GraphScanMinVersion = "3.0.0"
)

func DownloadIndexerIfNeeded(xrayManager *xray.XrayServicesManager) (string, error) {
	xrayVersionStr, err := xrayManager.GetVersion()
	if err != nil {
		return "", err
	}
	xrayVersion := version.NewVersion(xrayVersionStr)
	if !xrayVersion.AtLeast(GraphScanMinVersion) {
		return "", errorutils.CheckError(errors.New("You are using Xray version " + string(xrayVersion.GetVersion()) + " while this operation requires Xray version " + GraphScanMinVersion + " or higher."))
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
	return indexerPath, err
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
