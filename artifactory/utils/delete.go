package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	rtclientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func ConfirmDelete(pathsToDeleteReader *content.ContentReader) (bool, error) {
	length, err := pathsToDeleteReader.Length()
	if err != nil || length < 1 {
		return false, err
	}
	for resultItem := new(rtclientutils.ResultItem); pathsToDeleteReader.NextRecord(resultItem) == nil; resultItem = new(rtclientutils.ResultItem) {
		log.Output("  " + resultItem.GetItemRelativePath())
	}
	if err := pathsToDeleteReader.GetError(); err != nil {
		return false, err
	}
	pathsToDeleteReader.Reset()
	return coreutils.AskYesNo("Are you sure you want to delete the above paths?", false), nil
}

func CreateDeleteServiceManager(artDetails *config.ServerDetails, threads, httpRetries, httpRetryWaitMilliSecs int, dryRun bool) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithThreads(artDetails, dryRun, threads, httpRetries, httpRetryWaitMilliSecs)
}
