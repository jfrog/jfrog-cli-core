package terraform

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type TerraformCommand struct {
	repo          string
	serverDetails *config.ServerDetails
}

func (nc *TerraformCommand) SetServerDetails(serverDetails *config.ServerDetails) *TerraformCommand {
	nc.serverDetails = serverDetails
	return nc
}

func (nc *TerraformCommand) SetRepo(repo string) *TerraformCommand {
	nc.repo = repo
	return nc
}
