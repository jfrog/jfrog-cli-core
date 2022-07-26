package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type RepoType int

const (
	Local RepoType = iota
	Remote
	Virtual
	Federated
	ReleaseBundles
)

var RepoTypes = []string{
	"local",
	"remote",
	"virtual",
	"federated",
	"releaseBundles",
}

func (repoType RepoType) String() string {
	return RepoTypes[repoType]
}

func GetRepositories(artDetails *config.ServerDetails, repoType ...RepoType) ([]string, error) {
	sm, err := CreateServiceManager(artDetails, 3, 0, false)
	if err != nil {
		return nil, err
	}
	repos := []string{}
	for _, repoType := range repoType {
		filterParams := services.RepositoriesFilterParams{RepoType: repoType.String()}

		repositoriesDetails, err := sm.GetAllRepositoriesFiltered(filterParams)
		if err != nil {
			return nil, err
		}
		for _, repositoryDetails := range *repositoriesDetails {
			repos = append(repos, repositoryDetails.Key)
		}
	}

	return repos, nil
}

// Since we can't search dependencies in a remote repository, we will turn the search to the repository's cache.
// Local/Virtual repository name will be returned as is.
func GetRepoNameForDependenciesSearch(repoName string, serviceManager artifactory.ArtifactoryServicesManager) (string, error) {
	isRemote, err := IsRemoteRepo(repoName, serviceManager)
	if err != nil {
		return "", err
	}
	if isRemote {
		repoName += "-cache"
	}
	return repoName, err
}

func IsRemoteRepo(repoName string, serviceManager artifactory.ArtifactoryServicesManager) (bool, error) {
	repoDetails := &services.RepositoryDetails{}
	err := serviceManager.GetRepository(repoName, &repoDetails)
	if err != nil {
		return false, errorutils.CheckErrorf("failed to get details for repository '" + repoName + "'. Error:\n" + err.Error())
	}
	return repoDetails.GetRepoType() == "remote", nil
}
