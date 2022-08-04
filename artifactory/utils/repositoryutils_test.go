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
