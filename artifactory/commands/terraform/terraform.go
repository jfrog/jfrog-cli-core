package terraform

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type TerraformCommand struct {
	args          []string
	repo          string
	serverDetails *config.ServerDetails
}

func (tc *TerraformCommand) GetArgs() []string {
	return tc.args
}

func (tc *TerraformCommand) SetArgs(terraformArg []string) *TerraformCommand {
	tc.args = terraformArg
	return tc
}

func (nc *TerraformCommand) SetServerDetails(serverDetails *config.ServerDetails) *TerraformCommand {
	nc.serverDetails = serverDetails
	return nc
}

func (nc *TerraformCommand) SetRepo(repo string) *TerraformCommand {
	nc.repo = repo
	return nc
}
