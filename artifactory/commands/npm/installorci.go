package npm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const npmrcFileName = ".npmrc"
const npmrcBackupFileName = "jfrog.npmrc.backup"
const minSupportedNpmVersion = "5.4.0"

type NpmInstallOrCiCommand struct {
	configFilePath      string
	internalCommandName string
	threads             int
	CommonArgs
}

func NewNpmInstallCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{CommonArgs: CommonArgs{cmdName: "install"}, internalCommandName: "rt_npm_install"}
}

func NewNpmCiCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{CommonArgs: CommonArgs{cmdName: "ci"}, internalCommandName: "rt_npm_ci"}
}

func (nic *NpmInstallOrCiCommand) CommandName() string {
	return nic.internalCommandName
}

func (nic *NpmInstallOrCiCommand) SetConfigFilePath(configFilePath string) *NpmInstallOrCiCommand {
	nic.configFilePath = configFilePath
	return nic
}

func (nic *NpmInstallOrCiCommand) SetArgs(args []string) *NpmInstallOrCiCommand {
	nic.npmArgs = args
	return nic
}

func (nic *NpmInstallOrCiCommand) SetRepoConfig(conf *utils.RepositoryConfig) *NpmInstallOrCiCommand {
	serverDetails, _ := conf.ServerDetails()
	nic.SetRepo(conf.TargetRepo()).SetServerDetails(serverDetails)
	return nic
}

func (nic *NpmInstallOrCiCommand) Init() error {
	log.Info(fmt.Sprintf("Running npm %s.", nic.cmdName))
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
	threads, _, _, _, filteredNpmArgs, buildConfiguration, err := commandUtils.ExtractNpmOptionsFromArgs(nic.npmArgs)
	if err != nil {
		return err
	}
	nic.SetRepoConfig(resolverParams).SetArgs(filteredNpmArgs).SetThreads(threads).SetBuildConfiguration(buildConfiguration)
	return nil
}

func (nic *NpmInstallOrCiCommand) SetThreads(threads int) *NpmInstallOrCiCommand {
	nic.threads = threads
	return nic
}

func (nic *NpmInstallOrCiCommand) ServerDetails() (*config.ServerDetails, error) {
	return nic.serverDetails, nil
}

func (nic *NpmInstallOrCiCommand) Run() error {
	if err := nic.preparePrerequisites(nic.repo); err != nil {
		return err
	}

	if err := nic.createTempNpmrc(); err != nil {
		return nic.restoreNpmrcAndError(err)
	}

	if err := nic.runInstallOrCi(); err != nil {
		return nic.restoreNpmrcAndError(err)
	}

	if err := nic.restoreNpmrcFunc(); err != nil {
		return err
	}

	if !nic.collectBuildInfo {
		log.Info(fmt.Sprintf("npm %s finished successfully.", nic.cmdName))
		return nil
	}

	if err := nic.collectDependencies(); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("npm %s finished successfully.", nic.cmdName))
	return nil
}

func (nic *NpmInstallOrCiCommand) runInstallOrCi() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", nic.cmdName))
	filteredArgs := filterFlags(nic.npmArgs)
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          nic.executablePath,
		Command:      append([]string{nic.cmdName}, filteredArgs...),
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}

	if nic.collectBuildInfo && len(filteredArgs) > 0 {
		log.Warn("Build info dependencies collection with npm arguments is not supported. Build info creation will be skipped.")
		nic.collectBuildInfo = false
	}

	return errorutils.CheckError(gofrogcmd.RunCmd(npmCmdConfig))
}

func (nic *NpmInstallOrCiCommand) collectDependencies() error {
	nic.buildInfoModule.SetTypeRestriction(nic.typeRestriction)
	nic.buildInfoModule.SetNpmArgs(nic.npmArgs)

	serviceManager, err := utils.CreateServiceManager(nic.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	buildName, err := nic.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	previousBuildDependencies, err := commandUtils.GetDependenciesFromLatestBuild(serviceManager, buildName)
	if err != nil {
		return err
	}
	missingDepsChan := make(chan string)
	collectChecksumsFunc := createCollectChecksumsFunc(previousBuildDependencies, serviceManager, missingDepsChan)
	nic.buildInfoModule.SetTraverseDependenciesFunc(collectChecksumsFunc)
	nic.buildInfoModule.SetThreads(nic.threads)

	log.Info("Collecting dependencies information... For the first run of the build, this may take a few minutes. Subsequent runs should be faster.")

	var missingDependencies []string
	go func() {
		for depId := range missingDepsChan {
			missingDependencies = append(missingDependencies, depId)
		}
	}()

	if err = nic.buildInfoModule.CalcDependencies(); err != nil {
		return errorutils.CheckError(err)
	}
	close(missingDepsChan)
	printMissingDependencies(missingDependencies)
	return nil
}

func createCollectChecksumsFunc(previousBuildDependencies map[string]*buildinfo.Dependency, servicesManager artifactory.ArtifactoryServicesManager, missingDepsChan chan string) func(dependency *buildinfo.Dependency) (bool, error) {
	return func(dependency *buildinfo.Dependency) (bool, error) {
		splitDepId := strings.SplitN(dependency.Id, ":", 2)
		name := splitDepId[0]
		ver := splitDepId[1]

		// Get dependency info.
		checksum, fileType, err := commandUtils.GetDependencyInfo(name, ver, previousBuildDependencies, servicesManager)
		if err != nil || checksum == nil {
			missingDepsChan <- dependency.Id
			return false, err
		}

		// Update dependency.
		dependency.Type = fileType
		dependency.Checksum = checksum
		return true, nil
	}
}

func printMissingDependencies(missingDependencies []string) {
	if len(missingDependencies) == 0 {
		return
	}

	log.Warn(strings.Join(missingDependencies, "\n"), "\nThe npm dependencies above could not be found in Artifactory and therefore are not included in the build-info.\n"+
		"Deleting the local cache will force populating Artifactory with these dependencies.")
}

// Gets a config with value which is an array, and adds it to the conf list
func addArrayConfigs(conf []string, key, arrayValue string) []string {
	if arrayValue == "[]" {
		return conf
	}

	values := strings.TrimPrefix(strings.TrimSuffix(arrayValue, "]"), "[")
	valuesSlice := strings.Split(values, ",")
	for _, val := range valuesSlice {
		confToAdd := fmt.Sprintf("%s[] = %s", key, val)
		conf = append(conf, confToAdd, "\n")
	}

	return conf
}

func removeNpmrcIfExists(workingDirectory string) error {
	if _, err := os.Stat(filepath.Join(workingDirectory, npmrcFileName)); err != nil {
		if os.IsNotExist(err) { // The file dose not exist, nothing to do.
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
