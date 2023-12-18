package precheckrunner

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
)

func TestGetIllegalDockerRepositoryKeys(t *testing.T) {
	repositoryNamingCheck := RepositoryNamingCheck{
		selectedRepos: map[utils.RepoType][]services.RepositoryDetails{utils.Local: {
			{Key: "a.b-docker", PackageType: "docker"},
			{Key: "a.b-generic", PackageType: "generic"},
			{Key: "a_b-docker", PackageType: "docker"},
			{Key: "ab-docker", PackageType: "docker"},
			{Key: "ab-generic", PackageType: "generic"},
		}},
	}
	actualIllegalRepositories := repositoryNamingCheck.getIllegalRepositoryKeys()
	assert.ElementsMatch(t, []illegalRepositoryKeys{
		{RepoKey: "a.b-docker", Reason: illegalDockerRepositoryKeyReason},
		{RepoKey: "a_b-docker", Reason: illegalDockerRepositoryKeyReason},
	}, actualIllegalRepositories)
}
