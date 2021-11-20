package npm

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/artifactory"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"strings"
)

const npmrcFileName = ".npmrc"
const npmrcBackupFileName = "jfrog.npmrc.backup"
const minSupportedNpmVersion = "5.4.0"

type InstallCiArgs struct {
	threads      int
	dependencies map[string]*npmutils.Dependency
	CommonArgs
}

type NpmInstallOrCiCommand struct {
	configFilePath      string
	internalCommandName string
	*InstallCiArgs
}

func NewNpmInstallCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{InstallCiArgs: NewInstallCiArgs("install"), internalCommandName: "rt_npm_install"}
}

func NewNpmCiCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{InstallCiArgs: NewInstallCiArgs("ci"), internalCommandName: "rt_npm_ci"}
}

func (nic *NpmInstallOrCiCommand) CommandName() string {
	return nic.internalCommandName
}

func (nic *NpmInstallOrCiCommand) SetConfigFilePath(configFilePath string) *NpmInstallOrCiCommand {
	nic.configFilePath = configFilePath
	return nic
}

func (nic *NpmInstallOrCiCommand) SetArgs(args []string) *NpmInstallOrCiCommand {
	nic.InstallCiArgs.npmArgs = args
	return nic
}

func (nic *NpmInstallOrCiCommand) SetRepoConfig(conf *utils.RepositoryConfig) *NpmInstallOrCiCommand {
	serverDetails, _ := conf.ServerDetails()
	nic.InstallCiArgs.SetRepo(conf.TargetRepo()).SetServerDetails(serverDetails)
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

func (ica *InstallCiArgs) SetThreads(threads int) *InstallCiArgs {
	ica.threads = threads
	return ica
}

func (ica *InstallCiArgs) SetTypeRestriction(typeRestriction npmutils.TypeRestriction) *InstallCiArgs {
	ica.typeRestriction = typeRestriction
	return ica
}

func (ica *InstallCiArgs) SetPackageInfo(packageInfo *npmutils.PackageInfo) *InstallCiArgs {
	ica.packageInfo = packageInfo
	return ica
}

func NewInstallCiArgs(npmCommand string) *InstallCiArgs {
	return &InstallCiArgs{CommonArgs: CommonArgs{cmdName: npmCommand}}
}

func (ica *InstallCiArgs) ServerDetails() (*config.ServerDetails, error) {
	return ica.serverDetails, nil
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

	if err := nic.setDependenciesList(); err != nil {
		return err
	}

	if err := nic.collectDependenciesChecksums(); err != nil {
		return err
	}

	if err := nic.saveDependenciesData(); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("npm %s finished successfully.", nic.cmdName))
	return nil
}

func (ica *InstallCiArgs) runInstallOrCi() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", ica.cmdName))
	filteredArgs := filterFlags(ica.npmArgs)
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          ica.executablePath,
		Command:      append([]string{ica.cmdName}, filteredArgs...),
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}

	if ica.collectBuildInfo && len(filteredArgs) > 0 {
		log.Warn("Build info dependencies collection with npm arguments is not supported. Build info creation will be skipped.")
		ica.collectBuildInfo = false
	}

	return errorutils.CheckError(gofrogcmd.RunCmd(npmCmdConfig))
}

func (ica *InstallCiArgs) GetDependenciesList() map[string]*npmutils.Dependency {
	return ica.dependencies
}

func (ica *InstallCiArgs) collectDependenciesChecksums() error {
	log.Info("Collecting dependencies information... For the first run of the build, this may take a few minutes. Subsequent runs should be faster.")
	servicesManager, err := utils.CreateServiceManager(ica.serverDetails, -1, false)
	if err != nil {
		return err
	}

	previousBuildDependencies, err := commandUtils.GetDependenciesFromLatestBuild(servicesManager, ica.buildConfiguration.BuildName)
	if err != nil {
		return err
	}
	producerConsumer := parallel.NewBounedRunner(ica.threads, false)
	errorsQueue := clientutils.NewErrorsQueue(1)
	handlerFunc := ica.createGetDependencyInfoFunc(servicesManager, previousBuildDependencies)
	go func() {
		defer producerConsumer.Done()
		for i := range ica.dependencies {
			producerConsumer.AddTaskWithError(handlerFunc(i), errorsQueue.AddError)
		}
	}()
	producerConsumer.Run()
	return errorsQueue.GetError()
}

func (ica *InstallCiArgs) saveDependenciesData() error {
	log.Debug("Saving data.")
	if ica.buildConfiguration.Module == "" {
		ica.buildConfiguration.Module = ica.packageInfo.BuildInfoModuleId()
	}

	dependencies, missingDependencies := ica.transformDependencies()
	if err := commandUtils.SaveDependenciesData(dependencies, ica.buildConfiguration); err != nil {
		return err
	}

	commandUtils.PrintMissingDependencies(missingDependencies)
	return nil
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

func (ica *InstallCiArgs) setDependenciesList() (err error) {
	ica.dependencies, err = npmutils.CalculateDependenciesList(ica.typeRestriction, ica.npmArgs, ica.executablePath, ica.packageInfo.BuildInfoModuleId())
	return
}

// Creates a function that fetches dependency data.
// If a dependency was included in the previous build, take the checksums information from it.
// Otherwise, fetch the checksum from Artifactory.
// Can be applied from a producer-consumer mechanism.
func (ica *InstallCiArgs) createGetDependencyInfoFunc(servicesManager artifactory.ArtifactoryServicesManager,
	previousBuildDependencies map[string]*buildinfo.Dependency) getDependencyInfoFunc {
	return func(dependencyIndex string) parallel.TaskFunc {
		return func(threadId int) error {
			name := ica.dependencies[dependencyIndex].Name
			ver := ica.dependencies[dependencyIndex].Version

			// Get dependency info.
			checksum, fileType, err := commandUtils.GetDependencyInfo(name, ver, previousBuildDependencies, servicesManager, threadId)
			if err != nil || checksum == nil {
				return err
			}

			// Update dependency.
			ica.dependencies[dependencyIndex].FileType = fileType
			ica.dependencies[dependencyIndex].Checksum = checksum
			return nil
		}
	}
}

// Transforms the list of dependencies to buildinfo.Dependencies list and creates a list of dependencies that are missing in Artifactory.
func (ica *InstallCiArgs) transformDependencies() (dependencies []buildinfo.Dependency, missingDependencies []buildinfo.Dependency) {
	for _, dependency := range ica.dependencies {
		biDependency := buildinfo.Dependency{Id: dependency.Name + ":" + dependency.Version, Type: dependency.FileType,
			Scopes: dependency.Scopes, Checksum: dependency.Checksum, RequestedBy: dependency.PathToRoot}
		if dependency.Checksum != nil {
			dependencies = append(dependencies,
				biDependency)
		} else {
			missingDependencies = append(missingDependencies, biDependency)
		}
	}
	return
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

type getDependencyInfoFunc func(string) parallel.TaskFunc
