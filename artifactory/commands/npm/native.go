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

type NativeCommandArgs struct {
	CommonArgs
}

// NativeCommand represents any npm command which is not "install", "ci" or "publish".
type NativeCommand struct {
	configFilePath string
	*NativeCommandArgs
}

func NewNpmNativeCommand(cmdName string) *NativeCommand {
	return &NativeCommand{
		NativeCommandArgs: &NativeCommandArgs{CommonArgs: CommonArgs{cmdName: cmdName}},
	}
}

func (nnc *NativeCommand) CommandName() string {
	return "rt_npm_native"
}

func (nnc *NativeCommand) SetConfigFilePath(configFilePath string) *NativeCommand {
	nnc.configFilePath = configFilePath
	return nnc
}

func (nnc *NativeCommand) SetServerDetails(serverDetails *config.ServerDetails) *NativeCommand {
	nnc.serverDetails = serverDetails
	return nnc
}

func (nnc *NativeCommand) Init() error {
	// Filter out JFrog CLI's specific flags.
	_, _, _, _, filteredCmd, _, err := commandUtils.ExtractNpmOptionsFromArgs(nnc.npmArgs)
	if err != nil {
		return err
	}

	err = nnc.setServerDetailsAndRepo()
	if err != nil {
		return err
	}
	nnc.SetNpmArgs(filteredCmd).SetBuildConfiguration(&utils.BuildConfiguration{})
	return nil
}

func (nnc *NativeCommand) Run() error {
	if err := nnc.preparePrerequisites(nnc.repo); err != nil {
		return err
	}

	if err := nnc.createTempNpmrc(); err != nil {
		return nnc.restoreNpmrcAndError(err)
	}

	if err := nnc.runNpmNativeCommand(); err != nil {
		return nnc.restoreNpmrcAndError(err)
	}

	return nnc.restoreNpmrcFunc()
}

func (nnc *NativeCommand) setServerDetailsAndRepo() error {
	// Read config file.
	log.Debug("Preparing to read the config file", nnc.configFilePath)
	vConfig, err := utils.ReadConfigFile(nnc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}

	// Set server details if exists, with priority to deployer.
	for _, prefix := range []string{utils.ProjectConfigDeployerPrefix, utils.ProjectConfigResolverPrefix} {
		if vConfig.IsSet(prefix) {
			params, err := utils.GetRepoConfigByPrefix(nnc.configFilePath, prefix, vConfig)
			if err != nil {
				return err
			}
			rtDetails, err := params.ServerDetails()
			if err != nil {
				return errorutils.CheckError(err)
			}
			nnc.SetServerDetails(rtDetails)
			nnc.repo = params.TargetRepo()
			return nil
		}
	}
	return nil
}

func (nca *NativeCommandArgs) ServerDetails() (*config.ServerDetails, error) {
	return nca.serverDetails, nil
}

func (nca *NativeCommandArgs) runNpmNativeCommand() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", nca.cmdName))
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          nca.executablePath,
		Command:      nca.npmArgs,
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
	command := npmCmdConfig.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
