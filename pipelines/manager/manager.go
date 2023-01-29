package manager

import (
	utilsconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
)

// CreateServiceManager creates pipelines manager and set auth details
func CreateServiceManager(serviceDetails *utilsconfig.ServerDetails) (*pipelines.PipelinesServicesManager, error) {
	pipelinesDetails := *serviceDetails
	// Create pipelines authentication config
	pAuth, err := pipelinesDetails.CreatePipelinesAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(pAuth).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return pipelines.New(serviceConfig)
}
