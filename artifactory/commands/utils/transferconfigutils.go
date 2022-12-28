package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "password"
)

func CreateArtifactoryClientDetails(serviceManager artifactory.ArtifactoryServicesManager) (*httputils.HttpClientDetails, error) {
	config := serviceManager.GetConfig()
	if config == nil {
		return nil, errorutils.CheckErrorf("expected full config, but no configuration exists")
	}
	rtDetails := config.GetServiceDetails()
	if rtDetails == nil {
		return nil, errorutils.CheckErrorf("artifactory details not configured")
	}
	clientDetails := rtDetails.CreateHttpClientDetails()
	return &clientDetails, nil
}

// Check if there is a configured user using default credentials 'admin:password' by pinging Artifactory.
func IsDefaultCredentials(manager artifactory.ArtifactoryServicesManager, artifactoryUrl string) (bool, error) {
	// Check if admin is locked
	lockedUsers, err := manager.GetLockedUsers()
	if err != nil {
		return false, err
	}
	if slices.Contains(lockedUsers, defaultAdminUsername) {
		return false, nil
	}

	// Ping Artifactory with the default admin:password credentials
	artDetails := config.ServerDetails{ArtifactoryUrl: clientUtils.AddTrailingSlashIfNeeded(artifactoryUrl), User: defaultAdminUsername, Password: defaultAdminPassword}
	servicesManager, err := utils.CreateServiceManager(&artDetails, -1, 0, false)
	if err != nil {
		return false, err
	}

	// This cannot be executed with commands.Exec()! Doing so will cause usage report being sent with admin:password as well.
	if _, err = servicesManager.Ping(); err == nil {
		log.Output()
		log.Warn("The default 'admin:password' credentials are used by a configured user in your source platform.\n" +
			"Those credentials will be transferred to your SaaS target platform.")
		return true, nil
	}

	// If the password of the admin user is not the default one, reset the failed login attempts counter in Artifactory
	// by unlocking the user. We do that to avoid login suspension to the admin user.
	return false, manager.UnlockUser(defaultAdminUsername)
}
