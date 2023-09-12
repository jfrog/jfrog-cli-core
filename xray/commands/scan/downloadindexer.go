package scan

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	gofrogio "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
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
		return
	}
	indexerDirPath := filepath.Join(dependenciesPath, indexerDirName)
	indexerBinaryName := getIndexerBinaryName()
	indexerPath = filepath.Join(indexerDirPath, xrayVersionStr, indexerBinaryName)

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return
	}
	unlockFunc, err := lock.CreateLock(filepath.Join(locksDirPath, indexerDirName))
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		e := unlockFunc()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return
	}
	exists, err := fileutils.IsFileExists(indexerPath, false)
	if exists || err != nil {
		return
	}

	log.Info("JFrog Xray Indexer " + xrayVersionStr + " is not cached locally. Downloading it now...")
	indexerPath, err = downloadIndexer(xrayManager, indexerDirPath, indexerBinaryName)
	if err != nil {
		err = errors.New("failed while attempting to download Xray indexer: " + err.Error())
	}
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
		err = fileutils.RemoveTempDir(tempDirPath)
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
	resp, err := xrayManager.Client().DownloadFile(downloadFileDetails, "", &httpClientDetails, false, false)
	if err != nil {
		return "", fmt.Errorf("an error occurred while trying to download '%s':\n%s", url, err.Error())
	}
	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			err = fmt.Errorf(err.Error() + "\nHint: It appears that the credentials provided do not have sufficient permissions for JFrog Xray. This could be due to either incorrect credentials or limited permissions restricted to Artifactory only.")
		}
		return "", fmt.Errorf("failed while attempting to download '%s':\n%s", url, err.Error())
	}

	// Add execution permissions to the indexer
	indexerPath := filepath.Join(tempDirPath, indexerBinaryName)
	err = os.Chmod(indexerPath, 0777)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	indexerVersion, err := getIndexerVersion(indexerPath)
	if err != nil {
		return "", err
	}
	log.Info("The downloaded Xray Indexer version is " + indexerVersion)
	newDirPath := filepath.Join(indexerDirPath, indexerVersion)

	// In case of a hot upgrade of Xray in progress, the version of the downloaded indexer might be different from the Xray version we got above,
	// so the indexer we just downloaded may already exist.
	newDirExists, err := fileutils.IsDirExists(newDirPath, false)
	if err != nil {
		return "", err
	}
	if newDirExists {
		err = fileutils.RemoveTempDir(tempDirPath)
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
	output, err := gofrogio.RunCmdOutput(indexCmd)
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

	filesList, err := os.ReadDir(indexerDirPath)
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
