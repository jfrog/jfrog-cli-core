package repository

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type RepoUpdateCommand struct {
	RepoCommand
}

func NewRepoUpdateCommand() *RepoUpdateCommand {
	return &RepoUpdateCommand{}
}

func (ruc *RepoUpdateCommand) SetTemplatePath(path string) *RepoUpdateCommand {
	ruc.templatePath = path
	return ruc
}

func (ruc *RepoUpdateCommand) SetVars(vars string) *RepoUpdateCommand {
	ruc.vars = vars
	return ruc
}

func (ruc *RepoUpdateCommand) SetServerDetails(serverDetails *config.ServerDetails) *RepoUpdateCommand {
	ruc.serverDetails = serverDetails
	return ruc
}

func (ruc *RepoUpdateCommand) ServerDetails() (*config.ServerDetails, error) {
	return ruc.serverDetails, nil
}

func (ruc *RepoUpdateCommand) CommandName() string {
	return "rt_repo_update"
}

func (ruc *RepoUpdateCommand) Run() (err error) {
	return ruc.PerformRepoCmd(true)
}
