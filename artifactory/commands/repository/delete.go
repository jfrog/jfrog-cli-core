package repository

import (
	"path/filepath"
	"strings"

	rtUtils "github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
)

type RepoDeleteCommand struct {
	rtDetails   *config.ArtifactoryDetails
	repoPattern string
	quiet       bool
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

func (rdc *RepoDeleteCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *RepoDeleteCommand {
	rdc.rtDetails = rtDetails
	return rdc
}

func (rdc *RepoDeleteCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return rdc.rtDetails, nil
}

func (rdc *RepoDeleteCommand) CommandName() string {
	return "rt_repo_delete"
}

func (rdc *RepoDeleteCommand) Run() (err error) {
	servicesManager, err := rtUtils.CreateServiceManager(rdc.rtDetails, false)
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
