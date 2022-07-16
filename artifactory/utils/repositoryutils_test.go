package utils

import (
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFilterRepositoryNames(t *testing.T) {
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
	actualRepoNames, err := filterRepositoryNames(&repos, includePatterns, excludePatterns)
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedRepoNames, actualRepoNames)
}
