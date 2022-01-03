package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
)

func CreateTerraformServiceManager(serverDetails *config.ServerDetails, httpRetries int, dryRun bool) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManager(serverDetails, httpRetries, dryRun)
}

type TerraformConfiguration struct {
	Threads               int
	MinChecksumDeploySize int64
	ExplodeArchive        bool
}
