package transfer

import (
	"path"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

const buildInfoPackageType = "BuildInfo"

// GetFilteredRepositories returns the names of all repositories filtered by their names and types.
// includePatterns - patterns of repository names (can contain wildcards) to include in the results. A repository's name
// must match at least one of these patterns in order to be included in the results. If includePatterns' length is zero,
// all repositories are included.
// excludePatterns - patterns of repository names (can contain wildcards) to exclude from the results. A repository's name
// must NOT match any of these patterns in order to be included in the results.
// filesTransferRepos - true if should return only repositories for files transfer
func GetFilteredRepositories(servicesManager artifactory.ArtifactoryServicesManager, includePatterns, excludePatterns []string, filesTransferRepos bool) ([]string, error) {
	var repos *[]services.RepositoryDetails
	var err error

	if filesTransferRepos {
		repos, err = servicesManager.GetAllRepositoriesFiltered(services.RepositoriesFilterParams{RepoType: utils.Local.String()})
		if err != nil {
			return nil, err
		}
	} else {
		repos, err = servicesManager.GetAllRepositories()
		if err != nil {
			return nil, err
		}
	}

	storageInfo, err := servicesManager.StorageInfo(true)
	if err != nil {
		return nil, err
	}
	addSpecialRepositories(repos, storageInfo, !filesTransferRepos)
	return filterRepositories(repos, includePatterns, excludePatterns)
}

// Add Release Bundle and Build Info repositories to the input repositories details list
// repos - The repositories details list
// storageInfo - /api/storageinfo response
// includeBuildInfo - True if should add the build info repositories to the list
func addSpecialRepositories(repos *[]services.RepositoryDetails, storageInfo *clientUtils.StorageInfo, includeBuildInfo bool) {
	for _, repoSummary := range storageInfo.RepositoriesSummaryList {
		if isReleaseBundleRepository(&repoSummary) || (includeBuildInfo && isBuildInfoRepository(&repoSummary)) {
			*repos = append(*repos, services.RepositoryDetails{Key: repoSummary.RepoKey})
		}
	}
}

func isReleaseBundleRepository(repoSummary *clientUtils.RepositorySummary) bool {
	return repoSummary.RepoType == "NA" && repoSummary.PackageType == "NA"
}

func isBuildInfoRepository(repoSummary *clientUtils.RepositorySummary) bool {
	return repoSummary.PackageType == buildInfoPackageType
}

// Filter repositories by name and return a list of repository names
// repos - The repositories details list
// includePatterns - Repositories inclusion wildcard pattern
// excludePatterns - Repositories exclusion wildcard pattern
func filterRepositories(repos *[]services.RepositoryDetails, includePatterns, excludePatterns []string) ([]string, error) {
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
			matched, err := path.Match(includePattern, repo.Key)
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
				matched, err := path.Match(excludePattern, repo.Key)
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
