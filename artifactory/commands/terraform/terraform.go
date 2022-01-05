package terraform

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type TerraformCommand struct {
	args           []string
	repo           string
	configFilePath string
	serverDetails  *config.ServerDetails
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

func (nc *TerraformCommand) setRepoFromConfiguration() error {
	// Read config file.
	log.Debug("Preparing to read the config file", nc.configFilePath)
	vConfig, err := utils.ReadConfigFile(nc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}
	deployerParams, err := utils.GetRepoConfigByPrefix(nc.configFilePath, utils.ProjectConfigDeployerPrefix, vConfig)
	if err != nil {
		return err
	}
	nc.SetRepo(deployerParams.TargetRepo())
	return nil
}
