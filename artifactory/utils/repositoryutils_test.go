package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterRepositoryNames(t *testing.T) {
	repos := []string{
		"jfrog-docker-local",
		"jfrog-npm-local",
		"docker-local",
		"jfrog-generic-remote",
		"jfrog-maven-local",
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
	actualRepoNames, err := filterRepositoryNames(&repos, includePatterns, excludePatterns)
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedRepoNames, actualRepoNames)
}

var shouldIncludeRepositoryTestCases = []struct {
	name            string
	repoKey         string
	includePatterns []string
	excludePatterns []string
	shouldInclude   bool
}{
	{name: "Exact match", repoKey: "generic-local", includePatterns: []string{"generic-local"}, excludePatterns: []string{}, shouldInclude: true},
	{name: "Wildcard pattern", repoKey: "generic-local", includePatterns: []string{"*-local"}, excludePatterns: []string{}, shouldInclude: true},
	{name: "No match", repoKey: "generic-local", includePatterns: []string{"generic", "local"}, excludePatterns: []string{}, shouldInclude: false},
	{name: "Third match", repoKey: "generic-local", includePatterns: []string{"generic", "local", "generic-local"}, excludePatterns: []string{}, shouldInclude: true},
	{name: "Empty match", repoKey: "generic-local", includePatterns: []string{}, excludePatterns: []string{}, shouldInclude: true},
	{name: "All match", repoKey: "generic-local", includePatterns: []string{"*"}, excludePatterns: []string{}, shouldInclude: true},
	{name: "Exact exclude", repoKey: "generic-local", includePatterns: []string{}, excludePatterns: []string{"generic-local"}, shouldInclude: false},
	{name: "All exclude", repoKey: "generic-local", includePatterns: []string{}, excludePatterns: []string{"*"}, shouldInclude: false},
	{name: "All include and exclude", repoKey: "generic-local", includePatterns: []string{"*"}, excludePatterns: []string{"*"}, shouldInclude: false},
	{name: "Blacklisted", repoKey: "jfrog-logs", includePatterns: []string{}, excludePatterns: []string{}, shouldInclude: false},
}

func TestShouldIncludeRepository(t *testing.T) {
	for _, testCase := range shouldIncludeRepositoryTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			repositoryFilter := &IncludeExcludeFilter{IncludePatterns: testCase.includePatterns, ExcludePatterns: testCase.excludePatterns}
			actual, err := repositoryFilter.ShouldIncludeRepository(testCase.repoKey)
			assert.NoError(t, err)
			assert.Equal(t, testCase.shouldInclude, actual)
		})
	}
}
