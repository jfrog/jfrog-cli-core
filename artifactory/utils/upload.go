package utils

import (
	"os"
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io"
)

func CreateUploadServiceManager(serverDetails *config.ServerDetails, threads, httpRetries, httpRetryWaitMilliSecs int, dryRun bool, progressBar io.ProgressMgr) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithProgressBar(serverDetails, threads, httpRetries, httpRetryWaitMilliSecs, dryRun, progressBar)
}

type UploadConfiguration struct {
	Deb                   string
	Threads               int
	MinChecksumDeploySize int64
	ExplodeArchive        bool
	SplitCount            int
	MinSplitSizeMB        int64
}

func GetMinChecksumDeploySize() (int64, error) {
	minChecksumDeploySize := os.Getenv("JFROG_CLI_MIN_CHECKSUM_DEPLOY_SIZE_KB")
	if minChecksumDeploySize == "" {
		return services.DefaultMinChecksumDeploy, nil
	}
	minSize, err := strconv.ParseInt(minChecksumDeploySize, 10, 64)
	err = errorutils.CheckError(err)
	if err != nil {
		return 0, err
	}
	return minSize * 1000, nil
}
