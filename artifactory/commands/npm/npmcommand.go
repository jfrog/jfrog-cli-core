package npm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const npmrcFileName = ".npmrc"
const npmrcBackupFileName = "jfrog.npmrc.backup"
const minSupportedNpmVersion = "5.4.0"

type NpmCommand struct {
	cmdName             string
	configFilePath      string
	internalCommandName string
	executablePath      string
	collectBuildInfo    bool
	buildInfoModule     *build.NpmModule
	CommonArgs
}

func NewNpmCommand(cmdName string) *NpmCommand {
	return &NpmCommand{
		cmdName:    cmdName,
		CommonArgs: CommonArgs{cmdName: cmdName},
	}
}

func NewNpmInstallCommand() *NpmCommand {
	return &NpmCommand{CommonArgs: CommonArgs{cmdName: "install"}, internalCommandName: "rt_npm_install"}
}

func NewNpmCiCommand() *NpmCommand {
	return &NpmCommand{CommonArgs: CommonArgs{cmdName: "ci"}, internalCommandName: "rt_npm_ci"}
}

func (nic *NpmCommand) CommandName() string {
	return nic.internalCommandName
}

func (nic *NpmCommand) SetConfigFilePath(configFilePath string) *NpmCommand {
	nic.configFilePath = configFilePath
	return nic
}

func (nic *NpmCommand) SetArgs(args []string) *NpmCommand {
	nic.npmArgs = args
	return nic
}

func (nic *NpmCommand) SetRepoConfig(conf *utils.RepositoryConfig) *NpmCommand {
	serverDetails, _ := conf.ServerDetails()
	nic.SetRepo(conf.TargetRepo()).SetServerDetails(serverDetails)
	return nic
}

func (nic *NpmCommand) Init() error {
	// Read config file.
	log.Debug("Preparing to read the config file", nic.configFilePath)
	vConfig, err := utils.ReadConfigFile(nic.configFilePath, utils.YAML)
	if err != nil {
		return err
	}
	// Extract resolution params.
	resolverParams, err := utils.GetRepoConfigByPrefix(nic.configFilePath, utils.ProjectConfigResolverPrefix, vConfig)
	if err != nil {
		return err
	}
	_, _, _, filteredNpmArgs, buildConfiguration, err := commandUtils.ExtractNpmOptionsFromArgs(nic.npmArgs)
	if err != nil {
		return err
	}
	nic.SetRepoConfig(resolverParams).SetArgs(filteredNpmArgs).SetBuildConfiguration(buildConfiguration)
	return nil
}

func (nic *NpmCommand) ServerDetails() (*config.ServerDetails, error) {
	return nic.serverDetails, nil
}

func (nic *NpmCommand) Run() (err error) {
	if err = nic.PreparePrerequisites(nic.repo, true); err != nil {
		return
	}
	defer func() {
		e := nic.restoreNpmrcFunc()
		if err == nil {
			err = e
		}
	}()
	if err = nic.CreateTempNpmrc(); err != nil {
		return
	}

	if err = nic.prepareBuildInfoModule(); err != nil {
		return
	}

	if !nic.collectBuildInfo {
		log.Info(fmt.Sprintf("npm %s finished successfully.", nic.cmdName))
		return
	}
	if err = nic.collectDependencies(); err != nil {
		return
	}
	log.Info(fmt.Sprintf("npm %s finished successfully.", nic.cmdName))
	return
}

func (nic *NpmCommand) prepareBuildInfoModule() error {
	var err error
	nic.collectBuildInfo, err = nic.buildConfiguration.IsCollectBuildInfo()
	if err != nil || !nic.collectBuildInfo {
		return err
	}

	// Build-info should not be created when installing a single package (npm install <package name>).
	if len(filterFlags(nic.npmArgs)) > 0 {
		log.Info("Build-info dependencies collection is not supported for installations of single packages. Build-info creation is skipped.")
		nic.collectBuildInfo = false
		return nil
	}
	buildName, err := nic.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := nic.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	buildInfoService := utils.CreateBuildInfoService()
	npmBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, nic.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	nic.buildInfoModule, err = npmBuild.AddNpmModule(nic.workingDirectory)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if nic.buildConfiguration.GetModule() != "" {
		nic.buildInfoModule.SetName(nic.buildConfiguration.GetModule())
	}
	return nil
}

func (nic *NpmCommand) collectDependencies() error {
	nic.buildInfoModule.SetNpmArgs(append([]string{nic.cmdName}, nic.npmArgs...))
	return errorutils.CheckError(nic.buildInfoModule.Build())
}

// Gets a config with value which is an array
func addArrayConfigs(key, arrayValue string) string {
	if arrayValue == "[]" {
		return ""
	}

	values := strings.TrimPrefix(strings.TrimSuffix(arrayValue, "]"), "[")
	valuesSlice := strings.Split(values, ",")
	var configArrayValues strings.Builder
	for _, val := range valuesSlice {
		configArrayValues.WriteString(fmt.Sprintf("%s[] = %s\n", key, val))
	}

	return configArrayValues.String()
}

func removeNpmrcIfExists(workingDirectory string) error {
	if _, err := os.Stat(filepath.Join(workingDirectory, npmrcFileName)); err != nil {
		// The file does not exist, nothing to do.
		if os.IsNotExist(err) {
			return nil
		}
		return errorutils.CheckError(err)
	}

	log.Debug("Removing Existing .npmrc file")
	return errorutils.CheckError(os.Remove(filepath.Join(workingDirectory, npmrcFileName)))
}

// To avoid writing configurations that are used by us
func isValidKey(key string) bool {
	return !strings.HasPrefix(key, "//") &&
		!strings.HasPrefix(key, ";") && // Comments
		!strings.HasPrefix(key, "@") && // Scoped configurations
		key != "registry" &&
		key != "metrics-registry" &&
		key != "json" // Handled separately because 'npm c ls' should run with json=false
}

func filterFlags(splitArgs []string) []string {
	var filteredArgs []string
	for _, arg := range splitArgs {
		if !strings.HasPrefix(arg, "-") {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	return filteredArgs
}

/*// GenericCommand represents any npm command which is not "install", "ci" or "publish".
type GenericCommand struct {
	*CommonArgs
}

func NewNpmGenericCommand(cmdName string) *GenericCommand {
	return &GenericCommand{
		CommonArgs: &CommonArgs{cmdName: cmdName},
	}
}

func (gc *GenericCommand) CommandName() string {
	return "rt_npm_generic"
}

func (gc *GenericCommand) ServerDetails() (*config.ServerDetails, error) {
	return gc.serverDetails, nil
}

func (gc *GenericCommand) Run() (err error) {
	if err = gc.PreparePrerequisites("", false); err != nil {
		return
	}
	log.Debug(fmt.Sprintf("Running npm %s command.", gc.cmdName))
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          gc.executablePath,
		Command:      gc.npmArgs,
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
	command := npmCmdConfig.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}*/
