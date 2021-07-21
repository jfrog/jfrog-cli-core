package npm

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const npmrcFileName = ".npmrc"
const npmrcBackupFileName = "jfrog.npmrc.backup"
const minSupportedNpmVersion = "5.4.0"

type NpmCommandArgs struct {
	command          string
	threads          int
	jsonOutput       bool
	executablePath   string
	npmrcFileMode    os.FileMode
	workingDirectory string
	registry         string
	npmAuth          string
	collectBuildInfo bool
	dependencies     map[string]*npmutils.Dependency
	typeRestriction  npmutils.TypeRestriction
	authArtDetails   auth.ServiceDetails
	packageInfo      *coreutils.PackageInfo
	NpmCommand
}

type NpmInstallOrCiCommand struct {
	configFilePath      string
	internalCommandName string
	*NpmCommandArgs
}

func NewNpmInstallCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{NpmCommandArgs: NewNpmCommandArgs("install"), internalCommandName: "rt_npm_install"}
}

func NewNpmCiCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{NpmCommandArgs: NewNpmCommandArgs("ci"), internalCommandName: "rt_npm_ci"}
}

func (nic *NpmInstallOrCiCommand) CommandName() string {
	return nic.internalCommandName
}

func (nic *NpmInstallOrCiCommand) SetConfigFilePath(configFilePath string) *NpmInstallOrCiCommand {
	nic.configFilePath = configFilePath
	return nic
}

func (nic *NpmInstallOrCiCommand) SetArgs(args []string) *NpmInstallOrCiCommand {
	nic.NpmCommandArgs.npmArgs = args
	return nic
}

func (nic *NpmInstallOrCiCommand) SetRepoConfig(conf *utils.RepositoryConfig) *NpmInstallOrCiCommand {
	serverDetails, _ := conf.ServerDetails()
	nic.NpmCommandArgs.SetRepo(conf.TargetRepo()).SetServerDetails(serverDetails)
	return nic
}

func (nic *NpmInstallOrCiCommand) Run() error {
	log.Info(fmt.Sprintf("Running npm %s.", nic.command))
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
	threads, _, _, filteredNpmArgs, buildConfiguration, err := commandUtils.ExtractNpmOptionsFromArgs(nic.npmArgs)
	if err != nil {
		return err
	}
	nic.SetRepoConfig(resolverParams).SetArgs(filteredNpmArgs).SetThreads(threads).SetBuildConfiguration(buildConfiguration)
	return nic.run()
}

func (nca *NpmCommandArgs) SetThreads(threads int) *NpmCommandArgs {
	nca.threads = threads
	return nca
}

func (nca *NpmCommandArgs) SetTypeRestriction(typeRestriction npmutils.TypeRestriction) *NpmCommandArgs {
	nca.typeRestriction = typeRestriction
	return nca
}

func (nca *NpmCommandArgs) SetPackageInfo(packageInfo *coreutils.PackageInfo) *NpmCommandArgs {
	nca.packageInfo = packageInfo
	return nca
}

func NewNpmCommandArgs(npmCommand string) *NpmCommandArgs {
	return &NpmCommandArgs{command: npmCommand}
}

func (nca *NpmCommandArgs) ServerDetails() (*config.ServerDetails, error) {
	return nca.serverDetails, nil
}

func (nca *NpmCommandArgs) run() error {
	if err := nca.preparePrerequisites(nca.repo); err != nil {
		return err
	}

	if err := nca.createTempNpmrc(); err != nil {
		return nca.restoreNpmrcAndError(err)
	}

	if err := nca.runInstallOrCi(); err != nil {
		return nca.restoreNpmrcAndError(err)
	}

	if err := nca.restoreNpmrc(); err != nil {
		return err
	}

	if !nca.collectBuildInfo {
		log.Info(fmt.Sprintf("npm %s finished successfully.", nca.command))
		return nil
	}

	if err := nca.setDependenciesList(); err != nil {
		return err
	}

	if err := nca.collectDependenciesChecksums(); err != nil {
		return err
	}

	if err := nca.saveDependenciesData(); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("npm %s finished successfully.", nca.command))
	return nil
}

func (nca *NpmCommandArgs) preparePrerequisites(repo string) error {
	log.Debug("Preparing prerequisites.")
	path, err := npmutils.FindNpmExecutable()
	if err != nil {
		return err
	}
	nca.executablePath = path

	if err = nca.validateNpmVersion(); err != nil {
		return err
	}

	if err := nca.setJsonOutput(); err != nil {
		return err
	}

	nca.workingDirectory, err = coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	log.Debug("Working directory set to:", nca.workingDirectory)

	if err = nca.setArtifactoryAuth(); err != nil {
		return err
	}

	nca.npmAuth, nca.registry, err = commandUtils.GetArtifactoryNpmRepoDetails(repo, &nca.authArtDetails)
	if err != nil {
		return err
	}

	nca.collectBuildInfo, nca.packageInfo, err = commandUtils.PrepareBuildInfo(nca.workingDirectory, nca.buildConfiguration)
	if err != nil {
		return err
	}

	return nca.backupProjectNpmrc()
}

func (nca *NpmCommandArgs) setJsonOutput() error {
	jsonOutput, err := npm.ConfigGet(nca.npmArgs, "json", nca.executablePath)
	if err != nil {
		return err
	}

	// In case of --json=<not boolean>, the value of json is set to 'true', but the result from the command is not 'true'
	nca.jsonOutput = jsonOutput != "false"
	return nil
}

// In order to make sure the install/ci downloads the dependencies from Artifactory, we are creating a.npmrc file in the project's root directory.
// If such a file already exists, we are copying it aside.
// This method restores the backed up file and deletes the one created by the command.
func (nca *NpmCommandArgs) restoreNpmrc() (err error) {
	log.Debug("Restoring project .npmrc file")
	if err = os.Remove(filepath.Join(nca.workingDirectory, npmrcFileName)); err != nil {
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}
	log.Debug("Deleted the temporary .npmrc file successfully")

	if _, err = os.Stat(filepath.Join(nca.workingDirectory, npmrcBackupFileName)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}

	if err = ioutils.CopyFile(
		filepath.Join(nca.workingDirectory, npmrcBackupFileName),
		filepath.Join(nca.workingDirectory, npmrcFileName), nca.npmrcFileMode); err != nil {
		return errorutils.CheckError(err)
	}
	log.Debug("Restored project .npmrc file successfully")

	if err = os.Remove(filepath.Join(nca.workingDirectory, npmrcBackupFileName)); err != nil {
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}
	log.Debug("Deleted project", npmrcBackupFileName, "file successfully")
	return nil
}

func createRestoreErrorPrefix(workingDirectory string) string {
	return fmt.Sprintf("Error occurred while restoring project .npmrc file. "+
		"Delete '%s' and move '%s' (if exists) to '%s' in order to restore the project. Failure cause: \n",
		filepath.Join(workingDirectory, npmrcFileName),
		filepath.Join(workingDirectory, npmrcBackupFileName),
		filepath.Join(workingDirectory, npmrcFileName))
}

// In order to make sure the install/ci downloads the artifacts from Artifactory we create a .npmrc file in the project dir.
// If such a file exists we back it up as npmrcBackupFileName.
func (nca *NpmCommandArgs) createTempNpmrc() error {
	log.Debug("Creating project .npmrc file.")
	data, err := npm.GetConfigList(nca.npmArgs, nca.executablePath)
	configData, err := nca.prepareConfigData(data)
	if err != nil {
		return errorutils.CheckError(err)
	}

	if err = removeNpmrcIfExists(nca.workingDirectory); err != nil {
		return err
	}

	return errorutils.CheckError(ioutil.WriteFile(filepath.Join(nca.workingDirectory, npmrcFileName), configData, 0600))
}

func (nca *NpmCommandArgs) runInstallOrCi() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", nca.command))
	filteredArgs := filterFlags(nca.npmArgs)
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          nca.executablePath,
		Command:      append([]string{nca.command}, filteredArgs...),
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}

	if nca.collectBuildInfo && len(filteredArgs) > 0 {
		log.Warn("Build info dependencies collection with npm arguments is not supported. Build info creation will be skipped.")
		nca.collectBuildInfo = false
	}

	return errorutils.CheckError(gofrogcmd.RunCmd(npmCmdConfig))
}

func (nca *NpmCommandArgs) GetDependenciesList() map[string]*npmutils.Dependency {
	return nca.dependencies
}

func (nca *NpmCommandArgs) collectDependenciesChecksums() error {
	log.Info("Collecting dependencies information... For the first run of the build, this may take a few minutes. Subsequent runs should be faster.")
	servicesManager, err := utils.CreateServiceManager(nca.serverDetails, -1, false)
	if err != nil {
		return err
	}

	previousBuildDependencies, err := commandUtils.GetDependenciesFromLatestBuild(servicesManager, nca.buildConfiguration.BuildName)
	if err != nil {
		return err
	}
	producerConsumer := parallel.NewBounedRunner(nca.threads, false)
	errorsQueue := clientutils.NewErrorsQueue(1)
	handlerFunc := nca.createGetDependencyInfoFunc(servicesManager, previousBuildDependencies)
	go func() {
		defer producerConsumer.Done()
		for i := range nca.dependencies {
			producerConsumer.AddTaskWithError(handlerFunc(i), errorsQueue.AddError)
		}
	}()
	producerConsumer.Run()
	return errorsQueue.GetError()
}

func (nca *NpmCommandArgs) saveDependenciesData() error {
	log.Debug("Saving data.")
	if nca.buildConfiguration.Module == "" {
		nca.buildConfiguration.Module = nca.packageInfo.BuildInfoModuleId()
	}

	dependencies, missingDependencies := nca.transformDependencies()
	if err := commandUtils.SaveDependenciesData(dependencies, nca.buildConfiguration); err != nil {
		return err
	}

	commandUtils.PrintMissingDependencies(missingDependencies)
	return nil
}

func (nca *NpmCommandArgs) validateNpmVersion() error {
	npmVersion, err := npm.Version(nca.executablePath)
	if err != nil {
		return err
	}
	rtVersion := version.NewVersion(string(npmVersion))
	if rtVersion.Compare(minSupportedNpmVersion) > 0 {
		return errorutils.CheckError(errors.New(fmt.Sprintf(
			"JFrog CLI npm %s command requires npm client version "+minSupportedNpmVersion+" or higher", nca.command)))
	}
	return nil
}

// To make npm do the resolution from Artifactory we are creating .npmrc file in the project dir.
// If a .npmrc file already exists we will backup it and override while running the command
func (nca *NpmCommandArgs) backupProjectNpmrc() error {
	fileInfo, err := os.Stat(filepath.Join(nca.workingDirectory, npmrcFileName))
	if err != nil {
		if os.IsNotExist(err) {
			nca.npmrcFileMode = 0644
			return nil
		}
		return errorutils.CheckError(err)
	}

	nca.npmrcFileMode = fileInfo.Mode()
	src := filepath.Join(nca.workingDirectory, npmrcFileName)
	dst := filepath.Join(nca.workingDirectory, npmrcBackupFileName)
	if err = ioutils.CopyFile(src, dst, nca.npmrcFileMode); err != nil {
		return err
	}
	log.Debug("Project .npmrc file backed up successfully to", filepath.Join(nca.workingDirectory, npmrcBackupFileName))
	return nil
}

// This func transforms "npm config list" result to key=val list of values that can be set to .npmrc file.
// it filters any nil values key, changes registry and scope registries to Artifactory url and adds Artifactory authentication to the list
func (nca *NpmCommandArgs) prepareConfigData(data []byte) ([]byte, error) {
	var filteredConf []string
	configString := string(data)
	scanner := bufio.NewScanner(strings.NewReader(configString))

	for scanner.Scan() {
		currOption := scanner.Text()
		if currOption != "" {
			splitOption := strings.SplitN(currOption, "=", 2)
			key := strings.TrimSpace(splitOption[0])
			if len(splitOption) == 2 && isValidKey(key) {
				value := strings.TrimSpace(splitOption[1])
				if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
					filteredConf = addArrayConfigs(filteredConf, key, value)
				} else {
					filteredConf = append(filteredConf, currOption, "\n")
				}
				nca.setTypeRestriction(key, value)
			} else if strings.HasPrefix(splitOption[0], "@") {
				// Override scoped registries (@scope = xyz)
				filteredConf = append(filteredConf, splitOption[0], " = ", nca.registry, "\n")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errorutils.CheckError(err)
	}

	filteredConf = append(filteredConf, "json = ", strconv.FormatBool(nca.jsonOutput), "\n")
	filteredConf = append(filteredConf, "registry = ", nca.registry, "\n")
	filteredConf = append(filteredConf, nca.npmAuth)
	return []byte(strings.Join(filteredConf, "")), nil
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

func (nca *NpmCommandArgs) setTypeRestriction(key string, value string) {
	// From npm 7, type restriction is determined by 'omit' and 'include' (both appear in 'npm config ls').
	// Other options (like 'dev', 'production' and 'only') are deprecated, but if they're used anyway - 'omit' and 'include' are automatically calculated.
	// So 'omit' is always preferred, if it exists.
	if key == "omit" {
		if strings.Contains(value, "dev") {
			nca.typeRestriction = npmutils.ProdOnly
		} else {
			nca.typeRestriction = npmutils.All
		}
	} else if nca.typeRestriction == npmutils.DefaultRestriction { // Until npm 6, configurations in 'npm config ls' are sorted by priority in descending order, so typeRestriction should be set only if it was not set before
		if key == "only" {
			if strings.Contains(value, "prod") {
				nca.typeRestriction = npmutils.ProdOnly
			} else if strings.Contains(value, "dev") {
				nca.typeRestriction = npmutils.DevOnly
			}
		} else if key == "production" && strings.Contains(value, "true") {
			nca.typeRestriction = npmutils.ProdOnly
		}
	}
}

func (nca *NpmCommandArgs) setDependenciesList() (err error) {
	nca.dependencies, err = npmutils.CalculateDependenciesList(nca.typeRestriction, nca.npmArgs, nca.executablePath, nca.packageInfo.BuildInfoModuleId())
	return
}

// Creates a function that fetches dependency data.
// If a dependency was included in the previous build, take the checksums information from it.
// Otherwise, fetch the checksum from Artifactory.
// Can be applied from a producer-consumer mechanism.
func (nca *NpmCommandArgs) createGetDependencyInfoFunc(servicesManager artifactory.ArtifactoryServicesManager,
	previousBuildDependencies map[string]*buildinfo.Dependency) getDependencyInfoFunc {
	return func(dependencyIndex string) parallel.TaskFunc {
		return func(threadId int) error {
			name := nca.dependencies[dependencyIndex].Name
			ver := nca.dependencies[dependencyIndex].Version

			// Get dependency info.
			checksum, fileType, err := commandUtils.GetDependencyInfo(name, ver, previousBuildDependencies, servicesManager, threadId)
			if err != nil || checksum == nil {
				return err
			}

			// Update dependency.
			nca.dependencies[dependencyIndex].FileType = fileType
			nca.dependencies[dependencyIndex].Checksum = checksum
			return nil
		}
	}
}

// Transforms the list of dependencies to buildinfo.Dependencies list and creates a list of dependencies that are missing in Artifactory.
func (nca *NpmCommandArgs) transformDependencies() (dependencies []buildinfo.Dependency, missingDependencies []buildinfo.Dependency) {
	for _, dependency := range nca.dependencies {
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

func (nca *NpmCommandArgs) restoreNpmrcAndError(err error) error {
	if restoreErr := nca.restoreNpmrc(); restoreErr != nil {
		return errors.New(fmt.Sprintf("Two errors occurred:\n %s\n %s", restoreErr.Error(), err.Error()))
	}
	return err
}

func (nca *NpmCommandArgs) setArtifactoryAuth() error {
	authArtDetails, err := nca.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	if authArtDetails.GetSshAuthHeaders() != nil {
		return errorutils.CheckError(errors.New("SSH authentication is not supported in this command"))
	}
	nca.authArtDetails = authArtDetails
	return nil
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
