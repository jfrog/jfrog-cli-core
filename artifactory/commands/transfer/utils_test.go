package transfer

import (
	"testing"

	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

func TestFilterRepositories(t *testing.T) {
	repos := []services.RepositoryDetails{
		{Key: "jfrog-docker-local"},
		{Key: "jfrog-npm-local"},
		{Key: "docker-local"},
		{Key: "jfrog-generic-remote"},
		{Key: "jfrog-maven-local"},
	}
	includePatterns := []string{
		"jfrog-*-local",
		"jfrog-generic-remote",
	}
	excludePatterns := []string{
		"*docker*",
		"jfrog-maven-local",
	}
	expectedRepoNames := []string{
		"jfrog-npm-local",
		"jfrog-generic-remote",
	}
	actualRepoNames, err := filterRepositories(&repos, includePatterns, excludePatterns)
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedRepoNames, actualRepoNames)
}

func TestAddSpecialRepositoriesIncludeBuildInfo(t *testing.T) {
	testAddSpecialRepositories(t, true)
}

func TestAddSpecialRepositoriesExcludeBuildInfo(t *testing.T) {
	testAddSpecialRepositories(t, false)
}

func testAddSpecialRepositories(t *testing.T, includeBuildInfo bool) {
	// Repository details response from GetAllRepositoeries
	repositoryDetails := []services.RepositoryDetails{{Key: "old-repo"}}

	// Create storage info response with all of the possible repository types
	storageInfo := utils.StorageInfo{RepositoriesSummaryList: []utils.RepositorySummary{
		{RepoKey: "local-repo", RepoType: "LOCAL", PackageType: "Generic"},
		{RepoKey: "cache-repo", RepoType: "CACHE", PackageType: "Maven"},
		{RepoKey: "distribution-repo", RepoType: "NA", PackageType: "NA"},
		{RepoKey: "virtual-repo", RepoType: "VIRTUAL", PackageType: "Pypi"},
		{RepoKey: "buildinfo-repo", RepoType: "LOCAL", PackageType: buildInfoPackageType},
	}}

	// Make sure that the distribution and the build info repositories are added to the list
	expectedRepositoryDetails := []services.RepositoryDetails{
		{Key: "old-repo"},
		{Key: "distribution-repo"},
	}
	if includeBuildInfo {
		expectedRepositoryDetails = append(expectedRepositoryDetails, services.RepositoryDetails{Key: "buildinfo-repo"})
	}
	addSpecialRepositories(&repositoryDetails, &storageInfo, includeBuildInfo)
	assert.ElementsMatch(t, expectedRepositoryDetails, repositoryDetails)

	// Make sure that the added repositories are not filtered out in filterRepositories
	expectedRepoNames := []string{
		"old-repo",
		"distribution-repo",
	}
	if includeBuildInfo {
		expectedRepoNames = append(expectedRepoNames, "buildinfo-repo")
	}
	actualRepoNames, err := filterRepositories(&repositoryDetails, []string{}, []string{})
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedRepoNames, actualRepoNames)
}
