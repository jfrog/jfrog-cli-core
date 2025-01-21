package utils

import (
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"golang.org/x/exp/slices"
	"path"
	"strings"

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
	Unknown
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

func RepoTypeFromString(repoTypeStr string) RepoType {
	switch strings.ToLower(repoTypeStr) {
	case Local.String():
		return Local
	case Remote.String():
		return Remote
	case Virtual.String():
		return Virtual
	case Federated.String():
		return Federated
	}
	return Unknown
}

// System repositories in Artifactory to filter in filterRepositoryNames.
// This is important especially for transfer-config, to prevent rewriting these repositories, which causes unexpected exceptions.
var blacklistedRepositories = []string{
	"jfrog-usage-logs", "jfrog-billing-logs", "jfrog-logs", "artifactory-pipe-info", "auto-trashcan", "jfrog-support-bundle", "_intransit", "artifactory-edge-uploads",
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

// SelectRepositoryInteractively prompts the user to select a repository from a list of repositories that match the given filter parameters.
func SelectRepositoryInteractively(serverDetails *config.ServerDetails, repoFilterParams services.RepositoriesFilterParams, promptMessage string) (string, error) {
	sm, err := CreateServiceManager(serverDetails, 3, 0, false)
	if err != nil {
		return "", err
	}

	filteredRepos, err := GetFilteredRepositoriesWithFilterParams(sm, nil, nil,
		repoFilterParams)
	if err != nil {
		return "", err
	}

	if len(filteredRepos) == 0 {
		return "", errorutils.CheckErrorf("no repositories were found that match the following criteria: %v", repoFilterParams)
	}

	if len(filteredRepos) == 1 {
		// Automatically select the repository if only one exists.
		return filteredRepos[0], nil
	}
	// Prompt the user to select a repository.
	return ioutils.AskFromListWithMismatchConfirmation(promptMessage, "Repository not found.", ioutils.ConvertToSuggests(filteredRepos)), nil
}

// GetFilteredRepositoriesWithFilterParams returns the names of local, remote, virtual, and federated repositories filtered by their names and type.
// servicesManager - The Artifactory services manager used to interact with the Artifactory server.
// includePatterns - Patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - Patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
// filterParams - Parameters to filter the repositories by their type.
// Returns a slice of repository names that match the given patterns and type, or an error if the operation fails.
func GetFilteredRepositoriesWithFilterParams(servicesManager artifactory.ArtifactoryServicesManager, includePatterns, excludePatterns []string, filterParams services.RepositoriesFilterParams) ([]string, error) {
	repoDetailsList, err := servicesManager.GetAllRepositoriesFiltered(filterParams)
	if err != nil {
		return nil, err
	}

	repoKeys := make([]string, len(*repoDetailsList))
	for i, repoDetails := range *repoDetailsList {
		repoKeys[i] = repoDetails.Key
	}

	return filterRepositoryNames(repoKeys, includePatterns, excludePatterns)
}

// GetFilteredBuildInfoRepositories returns the names of all build-info repositories filtered by their names.
// storageInfo - storage info response from Artifactory
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
func GetFilteredBuildInfoRepositories(storageInfo *clientUtils.StorageInfo, includePatterns, excludePatterns []string) ([]string, error) {
	repoKeys := make([]string, 0, len(storageInfo.RepositoriesSummaryList))

	for _, repoSummary := range storageInfo.RepositoriesSummaryList {
		if strings.ToLower(repoSummary.PackageType) == buildInfoPackageType {
			repoKeys = append(repoKeys, repoSummary.RepoKey)
		}
	}
	return filterRepositoryNames(repoKeys, includePatterns, excludePatterns)
}

// Filter repositories by name and return a list of repository names
// repos - The repository keys to filter
// includePatterns - Repositories inclusion wildcard pattern
// excludePatterns - Repositories exclusion wildcard pattern
func filterRepositoryNames(repoKeys []string, includePatterns, excludePatterns []string) ([]string, error) {
	includedRepos := datastructures.MakeSet[string]()
	includeExcludeFilter := IncludeExcludeFilter{
		IncludePatterns: includePatterns,
		ExcludePatterns: excludePatterns,
	}
	for _, repoKey := range repoKeys {
		repoIncluded, err := includeExcludeFilter.ShouldIncludeRepository(repoKey)
		if err != nil {
			return nil, err
		}
		if repoIncluded {
			includedRepos.Add(repoKey)
		}
	}
	return includedRepos.ToSlice(), nil
}

type IncludeExcludeFilter struct {
	IncludePatterns []string
	ExcludePatterns []string
}

func (ief *IncludeExcludeFilter) ShouldIncludeRepository(repoKey string) (bool, error) {
	if slices.Contains(blacklistedRepositories, repoKey) {
		// This repository is blacklisted.
		return false, nil
	}
	return ief.ShouldIncludeItem(repoKey)
}

func (ief *IncludeExcludeFilter) ShouldIncludeItem(item string) (bool, error) {
	// If includePattens is empty, include all.
	repoIncluded := len(ief.IncludePatterns) == 0

	// Check if this item name matches any include pattern.
	for _, includePattern := range ief.IncludePatterns {
		matched, err := path.Match(includePattern, item)
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

	// Check if this item name matches any exclude pattern.
	for _, excludePattern := range ief.ExcludePatterns {
		matched, err := path.Match(excludePattern, item)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil
		}
	}

	return true, nil
}
