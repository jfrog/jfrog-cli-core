package npm

import (
	"fmt"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

type GenericCommandArgs struct {
	CommonArgs
}

// GenericCommand represents any npm command which is not "install", "ci" or "publish".
type GenericCommand struct {
	configFilePath string
	*GenericCommandArgs
}

func NewNpmGenericCommand(cmdName string) *GenericCommand {
	return &GenericCommand{
		GenericCommandArgs: &GenericCommandArgs{CommonArgs: CommonArgs{cmdName: cmdName}},
	}
}

func (gc *GenericCommand) CommandName() string {
	return "rt_npm_generic"
}

func (gc *GenericCommand) SetConfigFilePath(configFilePath string) *GenericCommand {
	gc.configFilePath = configFilePath
	return gc
}

func (gc *GenericCommand) SetServerDetails(serverDetails *config.ServerDetails) *GenericCommand {
	gc.serverDetails = serverDetails
	return gc
}

func (gc *GenericCommand) Init() error {
	// Filter out JFrog CLI's specific flags.
	_, _, _, filteredCmd, _, err := commandUtils.ExtractNpmOptionsFromArgs(gc.npmArgs)
	if err != nil {
		return err
	}

	err = gc.setServerDetailsAndRepo()
	if err != nil {
		return err
	}
	gc.SetNpmArgs(filteredCmd).SetBuildConfiguration(&utils.BuildConfiguration{})
	return nil
}

func (gc *GenericCommand) Run() (err error) {
	if err = gc.preparePrerequisites(gc.repo); err != nil {
		return
	}
	defer func() {
		e := gc.restoreNpmrcFunc()
		if err == nil {
			err = e
		}
	}()
	if err = gc.createTempNpmrc(); err != nil {
		return
	}
	err = gc.runNpmGenericCommand()
	return
}

func (gc *GenericCommand) setServerDetailsAndRepo() error {
	// Read config file.
	log.Debug("Preparing to read the config file", gc.configFilePath)
	vConfig, err := utils.ReadConfigFile(gc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}

	// Set server details if exists, with priority to deployer.
	for _, prefix := range []string{utils.ProjectConfigDeployerPrefix, utils.ProjectConfigResolverPrefix} {
		if vConfig.IsSet(prefix) {
			params, err := utils.GetRepoConfigByPrefix(gc.configFilePath, prefix, vConfig)
			if err != nil {
				return err
			}
			rtDetails, err := params.ServerDetails()
			if err != nil {
				return errorutils.CheckError(err)
			}
			gc.SetServerDetails(rtDetails)
			gc.repo = params.TargetRepo()
			return nil
		}
	}
	return nil
}

func (gca *GenericCommandArgs) ServerDetails() (*config.ServerDetails, error) {
	return gca.serverDetails, nil
}

func (gca *GenericCommandArgs) runNpmGenericCommand() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", gca.cmdName))
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          gca.executablePath,
		Command:      gca.npmArgs,
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
	command := npmCmdConfig.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
