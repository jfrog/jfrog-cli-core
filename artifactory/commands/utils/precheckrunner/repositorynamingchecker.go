package precheckrunner

import (
	"fmt"
	"strings"
	"time"

	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	repositoryNamingCheckName        = "Repositories naming"
	illegalDockerRepositoryKeyReason = "Docker repository keys in the SasS are not allowed to include '.' or '_' characters."
)

type illegalRepositoryKeys struct {
	RepoKey string `json:"repo_key,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Run repository naming check before transferring configuration from one Artifactory to another
type RepositoryNamingCheck struct {
	selectedRepos map[utils.RepoType][]services.RepositoryDetails
}

func NewRepositoryNamingCheck(selectedRepos map[utils.RepoType][]services.RepositoryDetails) *RepositoryNamingCheck {
	return &RepositoryNamingCheck{selectedRepos}
}

func (drc *RepositoryNamingCheck) Name() string {
	return repositoryNamingCheckName
}

func (drc *RepositoryNamingCheck) ExecuteCheck(args RunArguments) (passed bool, err error) {
	results := drc.getIllegalRepositoryKeys()
	if len(results) == 0 {
		return true, nil
	}

	return false, handleFailuresInRepositoryKeysRun(results)
}

func (drc *RepositoryNamingCheck) getIllegalRepositoryKeys() []illegalRepositoryKeys {
	var results []illegalRepositoryKeys
	for _, repositoriesOfType := range drc.selectedRepos {
		for _, repository := range repositoriesOfType {
			if strings.ToLower(repository.PackageType) == "docker" && strings.ContainsAny(repository.Key, "_.") {
				log.Debug("Found Docker repository with illegal characters:", repository.Key)
				results = append(results, illegalRepositoryKeys{
					RepoKey: repository.Key,
					Reason:  illegalDockerRepositoryKeyReason,
				})
			}
		}
	}
	return results
}

// Create CSV summary of all the files with illegal repository keys and log the result
func handleFailuresInRepositoryKeysRun(illegalDockerRepositoryKeys []illegalRepositoryKeys) (err error) {
	// Create summary
	csvPath, err := commandUtils.CreateCSVFile("illegal-repository-keys", illegalDockerRepositoryKeys, time.Now())
	if err != nil {
		log.Error("Couldn't create the illegal repository keys CSV file", err)
		return
	}
	// Log result
	log.Info(fmt.Sprintf("Found %d illegal repository keys. Check the summary CSV file in: %s", len(illegalDockerRepositoryKeys), csvPath))
	return
}
