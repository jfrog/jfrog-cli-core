package manager

import (
	utilsconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
)

/*
CreateServiceManager creates pipelines manager from jfrog-go-client
*/
func CreateServiceManager(serviceDetails *utilsconfig.ServerDetails) (*pipelines.PipelinesServicesManager, error) {
	pipelinesDetails := *serviceDetails
	pAuth, authErr := pipelinesDetails.CreatePipelinesAuthConfig()
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
	pipelinesMgr, err := pipelines.New(serviceConfig)
	if err != nil {
		clientlog.Error(err)
		return nil, err
	}
	return pipelinesMgr, nil
}
