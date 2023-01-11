package utils

import (
	"path"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const buildInfoPackageType = "buildinfo"

type RepoType int

const (
	Local RepoType = iota
	Remote
	Virtual
	Federated
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

// System repositories in Artifactory to filter in filterRepositoryNames.
// This is important especially for transfer-config, to prevent rewriting these repositories, which causes unexpected exceptions.
var blacklistedRepositories = []string{
	"jfrog-usage-logs", "jfrog-billing-logs", "jfrog-logs", "artifactory-pipe-info", "auto-trashcan", "jfrog-support-bundle", "_intransit", "artifactory-edge-uploads",
}

// GetRepositories returns the names of local, remote, virtual or federated repositories filtered by their type.
// artDetails - Artifactory server details
// repoTypes - Repository types to filter. If empty - return all repository types.
func GetRepositories(artDetails *config.ServerDetails, repoTypes ...RepoType) ([]string, error) {
	sm, err := CreateServiceManager(artDetails, 3, 0, false)
	if err != nil {
		return nil, err
	}
	if len(repoTypes) == 0 {
		return GetFilteredRepositoriesByName(sm, nil, nil)
	}
	repos := []string{}
	for _, repoType := range repoTypes {
		repoKey, err := GetFilteredRepositoriesByNameAndType(sm, nil, nil, repoType)
		if err != nil {
			return repos, err
		}
		repos = append(repos, repoKey...)
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

// GetFilteredRepositoriesByName returns the names of local, remote, virtual and federated repositories filtered by their names.
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
func GetFilteredRepositoriesByName(servicesManager artifactory.ArtifactoryServicesManager, includePatterns, excludePatterns []string) ([]string, error) {
	repoDetailsList, err := servicesManager.GetAllRepositories()
	if err != nil {
		return nil, err
	}

	return getFilteredRepositories(repoDetailsList, includePatterns, excludePatterns)
}

// GetFilteredRepositoriesByNameAndType returns the names of local, remote, virtual and federated repositories filtered by their names and type.
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
// repoType - only repositories of this type will be returned.
func GetFilteredRepositoriesByNameAndType(servicesManager artifactory.ArtifactoryServicesManager, includePatterns, excludePatterns []string, repoType RepoType) ([]string, error) {
	repoDetailsList, err := servicesManager.GetAllRepositoriesFiltered(services.RepositoriesFilterParams{RepoType: repoType.String()})
	if err != nil {
		return nil, err
	}

	return getFilteredRepositories(repoDetailsList, includePatterns, excludePatterns)
}

func getFilteredRepositories(repoDetailsList *[]services.RepositoryDetails, includePatterns, excludePatterns []string) ([]string, error) {
	var repoKeys []string
	for _, repoDetails := range *repoDetailsList {
		repoKeys = append(repoKeys, repoDetails.Key)
	}

	return filterRepositoryNames(&repoKeys, includePatterns, excludePatterns)
}

// GetFilteredBuildInfoRepositories returns the names of all build-info repositories filtered by their names.
// storageInfo - storage info response from Artifactory
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
func GetFilteredBuildInfoRepositories(storageInfo *clientUtils.StorageInfo, includePatterns, excludePatterns []string) ([]string, error) {
	var repoKeys []string
	for _, repoSummary := range storageInfo.RepositoriesSummaryList {
		if strings.ToLower(repoSummary.PackageType) == buildInfoPackageType {
			repoKeys = append(repoKeys, repoSummary.RepoKey)
		}
	}
	return filterRepositoryNames(&repoKeys, includePatterns, excludePatterns)
}

// Filter repositories by name and return a list of repository names
// repos - The repository keys to filter
// includePatterns - Repositories inclusion wildcard pattern
// excludePatterns - Repositories exclusion wildcard pattern
func filterRepositoryNames(repoKeys *[]string, includePatterns, excludePatterns []string) ([]string, error) {
	var included []string
	repoFilter := &RepositoryFilter{
		IncludePatterns: includePatterns,
		ExcludePatterns: excludePatterns,
	}
	for _, repoKey := range *repoKeys {
		repoIncluded, err := repoFilter.ShouldIncludeRepository(repoKey)
		if err != nil {
			return included, err
		}
		if repoIncluded {
			included = append(included, repoKey)
		}
	}
	return included, nil
}

type RepositoryFilter struct {
	IncludePatterns []string
	ExcludePatterns []string
}

func (rf *RepositoryFilter) ShouldIncludeRepository(repoKey string) (bool, error) {
	// If includePattens is empty, include all repositories.
	repoIncluded := len(rf.IncludePatterns) == 0

	// Check if this repository name matches any include pattern.
	for _, includePattern := range rf.IncludePatterns {
		matched, err := path.Match(includePattern, repoKey)
		if err != nil {
			return false, err
		}
		if matched {
			repoIncluded = true
			break
		}
	}

	if !repoIncluded {
		return false, nil
	}

	// Check if this repository name matches any exclude pattern.
	for _, excludePattern := range append(rf.ExcludePatterns, blacklistedRepositories...) {
		matched, err := path.Match(excludePattern, repoKey)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil
		}
	}

	return true, nil
}
