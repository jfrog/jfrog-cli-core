package replication

import (
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

type ReplicationDeleteCommand struct {
	serverDetails *config.ServerDetails
	repoKey       string
	quiet         bool
}

func NewReplicationDeleteCommand() *ReplicationDeleteCommand {
	return &ReplicationDeleteCommand{}
}

func (rdc *ReplicationDeleteCommand) SetRepoKey(repoKey string) *ReplicationDeleteCommand {
	rdc.repoKey = repoKey
	return rdc
}

func (rdc *ReplicationDeleteCommand) SetQuiet(quiet bool) *ReplicationDeleteCommand {
	rdc.quiet = quiet
	return rdc
}

func (rdc *ReplicationDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReplicationDeleteCommand {
	rdc.serverDetails = serverDetails
	return rdc
}

func (rdc *ReplicationDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return rdc.serverDetails, nil
}

func (rdc *ReplicationDeleteCommand) CommandName() string {
	return "rt_replication_delete"
}

func (rdc *ReplicationDeleteCommand) Run() (err error) {
	if !rdc.quiet && !coreutils.AskYesNo("Are you sure you want to delete the replication for  "+rdc.repoKey+" ?", false) {
		return nil
	}
	servicesManager, err := rtUtils.CreateServiceManager(rdc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return servicesManager.DeleteReplication(rdc.repoKey)
}
