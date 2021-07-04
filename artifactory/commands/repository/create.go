package repository

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type RepoCreateCommand struct {
	RepoCommand
}

func NewRepoCreateCommand() *RepoCreateCommand {
	return &RepoCreateCommand{}
}

func (rcc *RepoCreateCommand) SetTemplatePath(path string) *RepoCreateCommand {
	rcc.templatePath = path
	return rcc
}

func (rcc *RepoCreateCommand) SetVars(vars string) *RepoCreateCommand {
	rcc.vars = vars
	return rcc
}

func (rcc *RepoCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *RepoCreateCommand {
	rcc.serverDetails = serverDetails
	return rcc
}

func (rcc *RepoCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return rcc.serverDetails, nil
}

func (rcc *RepoCreateCommand) CommandName() string {
	return "rt_repo_create"
}

func (rcc *RepoCreateCommand) Run() (err error) {
	return rcc.PerformRepoCmd(false)
}
