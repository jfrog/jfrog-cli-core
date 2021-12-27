package project

import (
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

const (
	Local     = "local"
	Remote    = "remote"
	Virtual   = "virtual"
	RemoteUrl = "url"

	// Defaults Repositories
	MavenLocalDefaultName    = "default-maven-local"
	MavenRemoteDefaultName   = "default-maven-remote"
	MavenRemoteDefaultUrl    = "https://repo.maven.apache.org/maven2"
	MavenVirtualDefaultName  = "default-maven-virtual"
	GradleLocalDefaultName   = "default-gradle-local"
	GradleRemoteDefaultName  = "default-gradle-remote"
	GradleRemoteDefaultUrl   = "https://repo.maven.apache.org/maven2"
	GradleVirtualDefaultName = "default-gradle-virtual"
	NpmLocalDefaultName      = "default-npm-local"
	NpmRemoteDefaultName     = "default-npm-remote"
	NpmRemoteDefaultUrl      = "https://registry.npmjs.org"
	NpmVirtualDefaultName    = "default-npm-virtual"
	GoLocalDefaultName       = "default-go-local"
	GoRemoteDefaultName      = "default-go-remote"
	GoRemoteDefaultUrl       = "https://gocenter.io/"
	GoVirtualDefaultName     = "default-go-virtual"
	PypiLocalDefaultName     = "default-pypi-local"
	PypiRemoteDefaultName    = "default-pypi-remote"
	PypiRemoteDefaultUrl     = "https://files.pythonhosted.org"
	PypiVirtualDefaultName   = "default-pypi-virtual"
	NugetLocalDefaultName    = "default-nuget-local"
	NugetRemoteDefaultName   = "default-nuget-remote"
	NugetRemoteDefaultUrl    = "https://www.nuget.org/"
	NugetVirtualDefaultName  = "default-nuget-virtual"
)

var RepoDefaultName = map[coreutils.Technology]map[string]string{
	coreutils.Maven: {
		Local:     MavenLocalDefaultName,
		Remote:    MavenRemoteDefaultName,
		RemoteUrl: MavenRemoteDefaultUrl,
		Virtual:   MavenVirtualDefaultName,
	},
	coreutils.Gradle: {
		Local:     GradleLocalDefaultName,
		Remote:    GradleRemoteDefaultName,
		RemoteUrl: GradleRemoteDefaultUrl,
		Virtual:   GradleVirtualDefaultName,
	},
	coreutils.Npm: {
		Local:     NpmLocalDefaultName,
		Remote:    NpmRemoteDefaultName,
		RemoteUrl: NpmRemoteDefaultUrl,
		Virtual:   NpmVirtualDefaultName,
	},
	coreutils.Go: {
		Local:     GoLocalDefaultName,
		Remote:    GoRemoteDefaultName,
		RemoteUrl: GoRemoteDefaultUrl,
		Virtual:   GoVirtualDefaultName,
	},
	coreutils.Pypi: {
		Local:     PypiLocalDefaultName,
		Remote:    PypiRemoteDefaultName,
		RemoteUrl: PypiRemoteDefaultUrl,
		Virtual:   PypiVirtualDefaultName,
	},
}

func CreateDefaultLocalRepo(technologyType coreutils.Technology, serverId string) error {
	servicesManager, err := getServiceManager(serverId)
	if err != nil {
		return err
	}
	params := services.NewLocalRepositoryBaseParams()
	params.PackageType = string(technologyType)
	params.Key = RepoDefaultName[technologyType][Local]
	if repoExists(servicesManager, params.Key) {
		return nil
	}
	return servicesManager.CreateLocalRepositoryWithParams(params)
}

func CreateDefaultRemoteRepo(technologyType coreutils.Technology, serverId string) error {
	servicesManager, err := getServiceManager(serverId)
	if err != nil {
		return err
	}
	params := services.NewRemoteRepositoryBaseParams()
	params.PackageType = string(technologyType)
	params.Key = RepoDefaultName[technologyType][Remote]
	params.Url = RepoDefaultName[technologyType][RemoteUrl]
	if repoExists(servicesManager, params.Key) {
		return nil
	}
	return servicesManager.CreateRemoteRepositoryWithParams(params)
}

func CreateDefaultVirtualRepo(technologyType coreutils.Technology, serverId string) error {
	servicesManager, err := getServiceManager(serverId)
	if err != nil {
		return err
	}
	params := services.NewVirtualRepositoryBaseParams()
	params.PackageType = string(technologyType)
	params.Key = RepoDefaultName[technologyType][Virtual]
	params.Repositories = []string{RepoDefaultName[technologyType][Local], RepoDefaultName[technologyType][Remote]}
	params.DefaultDeploymentRepo = RepoDefaultName[technologyType][Local]
	if repoExists(servicesManager, params.Key) {
		return nil
	}
	return servicesManager.CreateVirtualRepositoryWithParams(params)
}

func getServiceManager(serverId string) (artifactory.ArtifactoryServicesManager, error) {
	serviceDetails, err := config.GetSpecificConfig(serverId, true, false)
	if err != nil {
		return nil, err
	}
	return rtUtils.CreateServiceManager(serviceDetails, -1, false)

}

// Check if default repository is already exists
func repoExists(servicesManager artifactory.ArtifactoryServicesManager, repoKey string) bool {
	repo := &services.RepositoryDetails{}
	_ = servicesManager.GetRepository(repoKey, repo)
	return repo.Key != ""
}
