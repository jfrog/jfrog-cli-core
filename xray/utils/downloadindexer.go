package utils

import (
	"fmt"
	"github.com/jfrog/gofrog/version"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
)

const (
	indexerDirName     = "xray-indexer"
	tempIndexerDirName = "temp"
)

func DownloadIndexerIfNeeded(xrayManager *xray.XrayServicesManager, xrayVersionStr string) (indexerPath string, err error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	indexerDirPath := filepath.Join(dependenciesPath, indexerDirName)
	indexerBinaryName := getIndexerBinaryName()
	indexerPath = filepath.Join(indexerDirPath, xrayVersionStr, indexerBinaryName)

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return "", err
	}
	lockFile, err := lock.CreateLock(filepath.Join(locksDirPath, "xray-indexer"))
	defer func() {
		e := lockFile.Unlock()
		if err == nil {
			err = e
		}
	}()
	exists, err := fileutils.IsFileExists(indexerPath, false)
	if exists || err != nil {
		return
	}

	log.Info("JFrog Xray Indexer is not cached locally. Downloading it now...")
	indexerPath, err = downloadIndexer(xrayManager, indexerDirPath, indexerBinaryName)
	return
}

func downloadIndexer(xrayManager *xray.XrayServicesManager, indexerDirPath, indexerBinaryName string) (string, error) {
	tempDirPath := filepath.Join(indexerDirPath, tempIndexerDirName)

	// Delete the temporary directory if it exists
	tempDirExists, err := fileutils.IsDirExists(tempDirPath, false)
	if err != nil {
		return "", err
	}
	if tempDirExists {
		err = os.RemoveAll(tempDirPath)
		if err != nil {
			return "", errorutils.CheckErrorf("Temporary download directory already exists, and can't be removed: %s\nRemove this directory manually and try again: %s", err.Error(), tempDirPath)
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
		LocalFileName: indexerBinaryName,
	}
	httpClientDetails := xrayManager.Config().GetServiceDetails().CreateHttpClientDetails()
	resp, err := xrayManager.Client().DownloadFile(downloadFileDetails, "", &httpClientDetails, false)
	if err == nil && resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", errorutils.CheckErrorf("%s received when attempting to download %s. An error occurred while trying to read the body of the response: %s", resp.Status, url, err.Error())
		}
		err = resp.Body.Close()
		if err != nil {
			return "", errorutils.CheckErrorf("%s received when attempting to download %s. An error occurred while trying to close the body of the response: %s", resp.Status, url, err.Error())
		}
		return "", errorutils.CheckErrorf("%s received when attempting to download %s\n%s", resp.Status, url, body)
	}

	// Add execution permissions to the indexer
	indexerPath := filepath.Join(tempDirPath, indexerBinaryName)
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

	return filepath.Join(newDirPath, indexerBinaryName), errorutils.CheckError(err)
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

func getIndexerBinaryName() string {
	switch runtime.GOOS {
	case "windows":
		return "indexer-app.exe"
	default:
		return "indexer-app"
	}
}
