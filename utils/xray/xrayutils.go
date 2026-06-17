package xray

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
)

// Options for creating an Xray service manager.
type XrayManagerOption func(f *xray.XrayServicesManager)

// Global reference to the project key, used for API endpoints that require it for authentication
func WithScopedProjectKey(projectKey string) XrayManagerOption {
	return func(f *xray.XrayServicesManager) {
		f.SetProjectKey(projectKey)
	}
}

func CreateXrayServiceManager(serverDetails *config.ServerDetails, options ...XrayManagerOption) (manager *xray.XrayServicesManager, err error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return
	}
	xrayDetails, err := serverDetails.CreateXrayAuthConfig()
	if err != nil {
		return
	}
	serviceConfig, err := clientconfig.NewConfigBuilder().
		SetServiceDetails(xrayDetails).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		Build()
	if err != nil {
		return
	}
	manager, err = xray.New(serviceConfig)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		option(manager)
	}
	return
}
