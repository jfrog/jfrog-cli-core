package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"path/filepath"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type RepoType int

const (
	LOCAL RepoType = iota
	REMOTE
	VIRTUAL
	FEDERATED
)

var RepoTypes = []string{
	"local",
	"remote",
	"virtual",
	"federated",
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
	for i := range repoType {
		r, err := GetFilteredRepositories(sm, nil, nil, repoType[i])
		if err != nil {
			return repos, err
		}
		if len(r) > 0 {
			repos = append(repos, r...)
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

// GetFilteredRepositories returns the names of all repositories filtered by their names and types.
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
// repoType - only repositories of this type will be returned. Only a maximum of ONE repository type can be filtered.
// If more than one type are provided, the repositories will be filtered by the first one only.
func GetFilteredRepositories(servicesManager artifactory.ArtifactoryServicesManager, includePatterns, excludePatterns []string, repoType ...RepoType) ([]string, error) {
	filterParams := services.RepositoriesFilterParams{}
	if len(repoType) > 0 {
		filterParams.RepoType = repoType[0].String()
	}
	repos, err := servicesManager.GetAllRepositoriesFiltered(filterParams)
	if err != nil {
		return nil, err
	}
	return filterRepositoryNames(repos, includePatterns, excludePatterns)
}

func filterRepositoryNames(repos *[]services.RepositoryDetails, includePatterns, excludePatterns []string) ([]string, error) {
	allIncluded := false
	// If includePattens is empty, include all repositories.
	if len(includePatterns) == 0 {
		allIncluded = true
	}

	var included []string
	for _, repo := range *repos {
		repoIncluded := allIncluded

		// Check if this repository name matches any include pattern.
		for _, includePattern := range includePatterns {
			matched, err := filepath.Match(includePattern, repo.Key)
			if err != nil {
				return nil, err
			}
			if matched {
				repoIncluded = true
				break
			}
		}
		if repoIncluded {
			// Check if this repository name matches any exclude pattern.
			for _, excludePattern := range excludePatterns {
				matched, err := filepath.Match(excludePattern, repo.Key)
				if err != nil {
					return nil, err
				}
				if matched {
					repoIncluded = false
					break
				}
			}
			if repoIncluded {
				included = append(included, repo.Key)
			}
		}
	}
	return included, nil
}
