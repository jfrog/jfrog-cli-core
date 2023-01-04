package manager

import (
	utilsconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
)

// CreateServiceManager creates pipelines manager and set auth details
func CreateServiceManager(serviceDetails *utilsconfig.ServerDetails) (*pipelines.PipelinesServicesManager, error) {
	pipelinesDetails := *serviceDetails
	pAuth, authErr := pipelinesDetails.CreatePipelinesAuthConfig() // create pipelines authentication config
	if authErr != nil {
		return nil, authErr
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(pAuth).
		SetDryRun(false).
		Build()
	if err != nil {
		clientlog.Error(err)
		return nil, err
	}
	pipelinesMgr, pipeErr := pipelines.New(serviceConfig)
	if pipeErr != nil {
		clientlog.Error(pipeErr)
		return nil, pipeErr
	}
	return pipelinesMgr, nil
}
