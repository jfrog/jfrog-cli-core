package utils

import (
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

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
	GraphScanMinVersion = "3.29.0"
	indexerDirName      = "xray-indexer"
	tempIndexerDirName  = "temp"
)

func DownloadIndexerIfNeeded(xrayManager *xray.XrayServicesManager) (string, error) {
	xrayVersionStr, err := xrayManager.GetVersion()
	if err != nil {
		return "", err
	}
	xrayVersion := version.NewVersion(xrayVersionStr)
	if !xrayVersion.AtLeast(GraphScanMinVersion) {
		return "", errorutils.CheckError(errors.New("You are using Xray version " +
			string(xrayVersion.GetVersion()) + ", while this operation requires Xray version " + GraphScanMinVersion + " or higher."))
	}

	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	indexerDirPath := filepath.Join(dependenciesPath, indexerDirName)
	indexerPath := filepath.Join(indexerDirPath, xrayVersionStr, indexerFileName)

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return "", err
	}
	lockFile, err := lock.CreateLock(filepath.Join(locksDirPath, "xray-indexer"))
	defer lockFile.Unlock()

	exists, err := fileutils.IsFileExists(indexerPath, false)
	if exists || err != nil {
		return indexerPath, err
	}

	log.Info("JFrog Xray Indexer is not cached locally. Downloading it now...")
	return downloadIndexer(xrayManager, indexerDirPath)
}

func downloadIndexer(xrayManager *xray.XrayServicesManager, indexerDirPath string) (string, error) {
	tempDirPath := filepath.Join(indexerDirPath, tempIndexerDirName)

	// Delete the temporary directory if it exists
	tempDirExists, err := fileutils.IsDirExists(tempDirPath, false)
	if err != nil {
		return "", err
	}
	if tempDirExists {
		err = os.RemoveAll(tempDirPath)
		if err != nil {
			return "", errorutils.CheckError(errors.New(fmt.Sprintf("Temporary download directory already exists, and can't be removed: %s\nRemove this directory manually and try again: %s", err.Error(), tempDirPath)))
		}
	}

	// Delete all old indexers, but the two newest
	err = deleteOldIndexers(indexerDirPath)
	if err != nil {
		return "", err
	}

	// Download the indexer from Xray to the temporary directory
	url := fmt.Sprintf("%sapi/v1/indexer-resources/download/%s/%s", xrayManager.Config().GetServiceDetails().GetUrl(), runtime.GOOS, runtime.GOARCH)
	downloadFileDetails := &httpclient.DownloadFileDetails{
		DownloadPath:  url,
		LocalPath:     tempDirPath,
		LocalFileName: indexerFileName,
	}
	httpClientDetails := xrayManager.Config().GetServiceDetails().CreateHttpClientDetails()
	resp, err := xrayManager.Client().DownloadFile(downloadFileDetails, "", &httpClientDetails, false)
	if err == nil && resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", errorutils.CheckError(errors.New(fmt.Sprintf("%s received when attempting to download %s. An error occurred while trying to read the body of the response: %s", resp.Status, url, err.Error())))
		}
		resp.Body.Close()
		return "", errorutils.CheckError(errors.New(fmt.Sprintf("%s received when attempting to download %s\n%s", resp.Status, url, body)))
	}

	// Add execution permissions to the indexer
	indexerPath := filepath.Join(tempDirPath, indexerFileName)
	err = os.Chmod(indexerPath, 0777)

	indexerVersion, err := getIndexerVersion(indexerPath)
	if err != nil {
		return "", err
	}
	newDirPath := filepath.Join(indexerDirPath, indexerVersion)

	// In case of a hot upgrade of Xray in progress, the version of the downloaded indexer might be different from the Xray version we got above,
	// so the indexer we just downloaded may already exist.
	newDirExists, err := fileutils.IsDirExists(newDirPath, false)
	if err != nil {
		return "", err
	}
	if newDirExists {
		err = os.RemoveAll(tempDirPath)
	} else {
		err = fileutils.MoveDir(tempDirPath, newDirPath)
	}

	return filepath.Join(newDirPath, indexerFileName), errorutils.CheckError(err)
}

func getIndexerVersion(indexerPath string) (string, error) {
	indexCmd := &coreutils.GeneralExecCmd{
		ExecPath: indexerPath,
		Command:  []string{"version"},
	}
	output, err := io.RunCmdOutput(indexCmd)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	splitOutput := strings.Split(output, " ")
	// The output of the command looks like: jfrog xray indexer-app version 1.2.3
	indexerVersion := strings.TrimSuffix(splitOutput[len(splitOutput)-1], "\n")
	return indexerVersion, nil
}

func deleteOldIndexers(indexerDirPath string) error {
	indexerDirExists, err := fileutils.IsDirExists(indexerDirPath, false)
	if !indexerDirExists || err != nil {
		return err
	}

	filesList, err := ioutil.ReadDir(indexerDirPath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	var dirsList []string
	for _, file := range filesList {
		if file.IsDir() {
			dirsList = append(dirsList, file.Name())
		}
	}

	if len(dirsList) <= 2 {
		return nil
	}

	sort.Slice(dirsList, func(i, j int) bool {
		currVersion := version.NewVersion(dirsList[i])
		return currVersion.AtLeast(dirsList[j])
	})

	for i := 2; i < len(dirsList); i++ {
		err = os.RemoveAll(filepath.Join(indexerDirPath, dirsList[i]))
		if err != nil {
			return errorutils.CheckError(err)
		}
	}

	return nil
}
