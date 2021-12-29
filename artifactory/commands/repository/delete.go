package repository

import (
	"path/filepath"
	"strings"

	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
)

type RepoDeleteCommand struct {
	serverDetails *config.ServerDetails
	repoPattern   string
	quiet         bool
}

func NewRepoDeleteCommand() *RepoDeleteCommand {
	return &RepoDeleteCommand{}
}

func (rdc *RepoDeleteCommand) SetRepoPattern(repoPattern string) *RepoDeleteCommand {
	rdc.repoPattern = repoPattern
	return rdc
}

func (rdc *RepoDeleteCommand) SetQuiet(quiet bool) *RepoDeleteCommand {
	rdc.quiet = quiet
	return rdc
}

func (rdc *RepoDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *RepoDeleteCommand {
	rdc.serverDetails = serverDetails
	return rdc
}

func (rdc *RepoDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return rdc.serverDetails, nil
}

func (rdc *RepoDeleteCommand) CommandName() string {
	return "rt_repo_delete"
}

func (rdc *RepoDeleteCommand) Run() (err error) {
	servicesManager, err := rtUtils.CreateServiceManager(rdc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}

	// A single repo to be deleted
	if !strings.Contains(rdc.repoPattern, "*") {
		return rdc.deleteRepo(&servicesManager, rdc.repoPattern)
	}

	// A pattern for the repo name was received
	repos, err := servicesManager.GetAllRepositories()
	if err != nil {
		return err
	}
	for _, repo := range *repos {
		matched, err := filepath.Match(rdc.repoPattern, repo.Key)
		if err != nil {
			return err
		}
		if matched {
			if err := rdc.deleteRepo(&servicesManager, repo.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rdc *RepoDeleteCommand) deleteRepo(servicesManager *artifactory.ArtifactoryServicesManager, repoKey string) error {
	if !rdc.quiet && !coreutils.AskYesNo("Are you sure you want to permanently delete the repository "+repoKey+" including all of its content?", false) {
		return nil
	}
	return (*servicesManager).DeleteRepository(repoKey)
}
