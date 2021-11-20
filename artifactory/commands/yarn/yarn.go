package yarn

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"

	"github.com/jfrog/gofrog/parallel"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const yarnrcFileName = ".yarnrc.yml"
const yarnrcBackupFileName = "jfrog.yarnrc.backup"
const minSupportedYarnVersion = "2.4.0"
const npmScopesConfigName = "npmScopes"

type YarnCommand struct {
	executablePath     string
	workingDirectory   string
	registry           string
	npmAuthIdent       string
	repo               string
	collectBuildInfo   bool
	configFilePath     string
	yarnArgs           []string
	threads            int
	restoreYarnrcFunc  func() error
	packageInfo        *npmutils.PackageInfo
	serverDetails      *config.ServerDetails
	authArtDetails     auth.ServiceDetails
	buildConfiguration *utils.BuildConfiguration
	dependencies       map[string]*buildinfo.Dependency
	envVarsBackup      map[string]*string
}

func NewYarnCommand() *YarnCommand {
	return &YarnCommand{}
}

func (yc *YarnCommand) SetConfigFilePath(configFilePath string) *YarnCommand {
	yc.configFilePath = configFilePath
	return yc
}

func (yc *YarnCommand) SetArgs(args []string) *YarnCommand {
	yc.yarnArgs = args
	return yc
}

func (yc *YarnCommand) Run() error {
	log.Info("Running Yarn...")
	var err error
	if err = yc.validateSupportedCommand(); err != nil {
		return err
	}

	if err = yc.readConfigFile(); err != nil {
		return err
	}

	var filteredYarnArgs []string
	yc.threads, _, _, _, filteredYarnArgs, yc.buildConfiguration, err = commandUtils.ExtractNpmOptionsFromArgs(yc.yarnArgs)
	if err != nil {
		return err
	}

	if err = yc.preparePrerequisites(); err != nil {
		return err
	}

	yc.restoreYarnrcFunc, err = commandUtils.BackupFile(filepath.Join(yc.workingDirectory, yarnrcFileName), filepath.Join(yc.workingDirectory, yarnrcBackupFileName))
	if err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	if err = yc.modifyYarnConfigurations(); err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	if err = yarn.RunCustomCmd(filteredYarnArgs, yc.executablePath); err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	if err = yc.restoreConfigurationsFromBackup(); err != nil {
		return err
	}

	if yc.collectBuildInfo {
		if err = yc.setDependenciesList(); err != nil {
			return err
		}

		if err := yc.saveDependenciesData(); err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("Yarn finished successfully."))
	return nil
}

func (yc *YarnCommand) ServerDetails() (*config.ServerDetails, error) {
	return yc.serverDetails, nil
}

func (yc *YarnCommand) CommandName() string {
	return "rt_yarn"
}

func (yc *YarnCommand) validateSupportedCommand() error {
	for index, arg := range yc.yarnArgs {
		if arg == "npm" && len(yc.yarnArgs) > index {
			npmCommand := yc.yarnArgs[index+1]
			// The command 'yarn npm publish' is not supported
			if npmCommand == "publish" {
				return errorutils.CheckErrorf("The command 'jfrog rt yarn npm publish' is not supported. Use 'jfrog rt upload' instead.")
			}
			// 'yarn npm *' commands other than 'info' and 'whoami' are not supported
			if npmCommand != "info" && npmCommand != "whoami" {
				return errorutils.CheckErrorf("The command 'jfrog rt yarn npm %s' is not supported.", npmCommand)
			}
		}
	}
	return nil
}

func (yc *YarnCommand) readConfigFile() error {
	log.Debug("Preparing to read the config file", yc.configFilePath)
	vConfig, err := utils.ReadConfigFile(yc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}

	// Extract resolution params
	resolverParams, err := utils.GetRepoConfigByPrefix(yc.configFilePath, utils.ProjectConfigResolverPrefix, vConfig)
	if err != nil {
		return err
	}
	yc.repo = resolverParams.TargetRepo()
	yc.serverDetails, err = resolverParams.ServerDetails()
	return err
}

func (yc *YarnCommand) preparePrerequisites() error {
	log.Debug("Preparing prerequisites.")
	var err error
	if err = yc.setYarnExecutable(); err != nil {
		return err
	}

	if err = yc.validateYarnVersion(); err != nil {
		return err
	}

	yc.workingDirectory, err = coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	log.Debug("Working directory set to:", yc.workingDirectory)

	if err = yc.setArtifactoryAuth(); err != nil {
		return err
	}

	var npmAuthOutput string
	npmAuthOutput, yc.registry, err = commandUtils.GetArtifactoryNpmRepoDetails(yc.repo, &yc.authArtDetails)
	if err != nil {
		return err
	}
	yc.npmAuthIdent, err = extractAuthIdentFromNpmAuth(npmAuthOutput)
	if err != nil {
		return err
	}

	yc.collectBuildInfo, yc.packageInfo, err = commandUtils.PrepareBuildInfo(yc.workingDirectory, yc.buildConfiguration, nil)
	if err != nil {
		return err
	}

	return nil
}

func (yc *YarnCommand) setYarnExecutable() error {
	yarnExecPath, err := exec.LookPath("yarn")
	if err != nil {
		return errorutils.CheckError(err)
	}

	yc.executablePath = yarnExecPath
	log.Debug("Found Yarn executable at:", yc.executablePath)
	return nil
}

func (yc *YarnCommand) validateYarnVersion() error {
	yarnVersionStr, err := yarn.Version(yc.executablePath)
	if err != nil {
		return err
	}
	yarnVersion := version.NewVersion(yarnVersionStr)
	if yarnVersion.Compare(minSupportedYarnVersion) > 0 {
		return errorutils.CheckErrorf(
			"JFrog CLI yarn command requires Yarn version " + minSupportedYarnVersion + " or higher")
	}
	return nil
}

func (yc *YarnCommand) setArtifactoryAuth() error {
	authArtDetails, err := yc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	if authArtDetails.GetSshAuthHeaders() != nil {
		return errorutils.CheckErrorf("SSH authentication is not supported in this command")
	}
	yc.authArtDetails = authArtDetails
	return nil
}

func (yc *YarnCommand) restoreConfigurationsFromBackup() error {
	if err := yc.restoreEnvironmentVariables(); err != nil {
		return err
	}
	return yc.restoreYarnrcFunc()
}

func (yc *YarnCommand) restoreConfigurationsAndError(err error) error {
	if restoreErr := yc.restoreConfigurationsFromBackup(); restoreErr != nil {
		return errors.New(fmt.Sprintf("Two errors occurred:\n%s\n%s", restoreErr.Error(), err.Error()))
	}
	return err
}

func (yc *YarnCommand) restoreEnvironmentVariables() error {
	for key, value := range yc.envVarsBackup {
		if value == nil {
			if err := os.Unsetenv(key); err != nil {
				return err
			}
			continue
		}

		if err := os.Setenv(key, *value); err != nil {
			return err
		}
	}
	return nil
}

func (yc *YarnCommand) modifyYarnConfigurations() error {
	yc.envVarsBackup = make(map[string]*string)

	if err := yc.backupAndSetEnvironmentVariable("YARN_NPM_REGISTRY_SERVER", yc.registry); err != nil {
		return err
	}

	if err := yc.backupAndSetEnvironmentVariable("YARN_NPM_AUTH_IDENT", yc.npmAuthIdent); err != nil {
		return err
	}

	if err := yc.backupAndSetEnvironmentVariable("YARN_NPM_ALWAYS_AUTH", "true"); err != nil {
		return err
	}

	// Update scoped registries (these cannot be set in environment variables)
	npmScopesStr, err := yarn.ConfigGet(npmScopesConfigName, yc.executablePath, true)
	if err != nil {
		return err
	}
	npmScopesMap := make(map[string]yarnNpmScope)
	err = json.Unmarshal([]byte(npmScopesStr), &npmScopesMap)
	if err != nil {
		return errorutils.CheckError(err)
	}
	artifactoryScope := yarnNpmScope{NpmAlwaysAuth: true, NpmAuthIdent: yc.npmAuthIdent, NpmRegistryServer: yc.registry}
	for scopeName := range npmScopesMap {
		npmScopesMap[scopeName] = artifactoryScope
	}
	updatedNpmScopesStr, err := json.Marshal(npmScopesMap)
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = yarn.ConfigSet(npmScopesConfigName, string(updatedNpmScopesStr), yc.executablePath, true)
	return errorutils.CheckError(err)
}

type yarnNpmScope struct {
	NpmAlwaysAuth     bool   `json:"npmAlwaysAuth,omitempty"`
	NpmAuthIdent      string `json:"npmAuthIdent,omitempty"`
	NpmRegistryServer string `json:"npmRegistryServer,omitempty"`
}

func (yc *YarnCommand) backupAndSetEnvironmentVariable(key, value string) error {
	oldVal, exist := os.LookupEnv(key)
	if exist {
		yc.envVarsBackup[key] = &oldVal
	} else {
		yc.envVarsBackup[key] = nil
	}

	return errorutils.CheckError(os.Setenv(key, value))
}

// Run 'yarn info' and parse the returned JSON
func (yc *YarnCommand) setDependenciesList() error {
	// Run 'yarn info'
	responseStr, err := yarn.Info(yc.executablePath)
	if err != nil {
		log.Warn("An error was thrown while collecting dependencies info:", err.Error())
		// A returned error doesn't necessarily mean that the operation totally failed. If, in addition, the response is empty, then it probably does.
		if responseStr == "" {
			return err
		}
	}

	dependenciesMap := make(map[string]*YarnDependency)
	scanner := bufio.NewScanner(strings.NewReader(responseStr))
	packageName := yc.packageInfo.FullName()
	var root *YarnDependency

	for scanner.Scan() {
		var currDependency YarnDependency
		currDepBytes := scanner.Bytes()
		err = json.Unmarshal(currDepBytes, &currDependency)
		if err != nil {
			return errorutils.CheckError(err)
		}
		dependenciesMap[currDependency.Value] = &currDependency

		// Check whether this dependency's name starts with the package name (which means this is the root)
		if strings.HasPrefix(currDependency.Value, packageName+"@") {
			root = &currDependency
		}
	}

	servicesManager, err := utils.CreateServiceManager(yc.serverDetails, -1, false)
	if err != nil {
		return err
	}

	// Collect checksums from last build to decrease requests to Artifactory
	previousBuildDependencies, err := commandUtils.GetDependenciesFromLatestBuild(servicesManager, yc.buildConfiguration.BuildName)
	if err != nil {
		return err
	}
	yc.dependencies = make(map[string]*buildinfo.Dependency)

	log.Info("Collecting dependencies information... For the first run of the build, this may take a few minutes. Subsequent runs should be faster.")
	producerConsumer := parallel.NewBounedRunner(yc.threads, false)
	errorsQueue := clientutils.NewErrorsQueue(1)

	go func() {
		defer producerConsumer.Done()
		yc.appendDependencyRecursively(root, []string{}, dependenciesMap, previousBuildDependencies, servicesManager, producerConsumer, errorsQueue)
	}()

	producerConsumer.Run()
	return errorsQueue.GetError()
}

func (yc *YarnCommand) appendDependencyRecursively(yarnDependency *YarnDependency, pathToRoot []string, dependenciesMap map[string]*YarnDependency,
	previousBuildDependencies map[string]*buildinfo.Dependency, servicesManager artifactory.ArtifactoryServicesManager,
	producerConsumer parallel.Runner, errorsQueue *clientutils.ErrorsQueue) error {
	name := yarnDependency.Name()
	var version string
	if len(pathToRoot) == 0 {
		// The version of the local project returned from 'yarn info' is '0.0.0-use.local', but we need the version mentioned in package.json
		version = yc.packageInfo.Version
	} else {
		version = yarnDependency.Details.Version
	}
	id := name + ":" + version

	// To avoid infinite loops in case of circular dependencies, the dependency won't be added if it's already in pathToRoot
	if coreutils.StringsSliceContains(pathToRoot, id) {
		return nil
	}

	for _, dependencyPtr := range yarnDependency.Details.Dependencies {
		innerDepKey := getYarnDependencyKeyFromLocator(dependencyPtr.Locator)
		innerYarnDep, exist := dependenciesMap[innerDepKey]
		if !exist {
			return errorutils.CheckErrorf("An error occurred while creating dependencies tree: dependency %s was not found.", dependencyPtr.Locator)
		}
		yc.appendDependencyRecursively(innerYarnDep, append([]string{id}, pathToRoot...), dependenciesMap,
			previousBuildDependencies, servicesManager, producerConsumer, errorsQueue)
	}

	// The root project should not be added to the dependencies list
	if len(pathToRoot) == 0 {
		return nil
	}

	buildinfoDependency, exist := yc.dependencies[id]
	if !exist {
		buildinfoDependency = &buildinfo.Dependency{Id: id}
		yc.dependencies[id] = buildinfoDependency
		taskFunc := func(threadId int) error {
			checksum, fileType, err := commandUtils.GetDependencyInfo(name, version, previousBuildDependencies, servicesManager, threadId)
			if err != nil {
				return err
			}
			buildinfoDependency.Type = fileType
			buildinfoDependency.Checksum = checksum
			return nil
		}
		producerConsumer.AddTaskWithError(taskFunc, errorsQueue.AddError)
	}

	buildinfoDependency.RequestedBy = append(buildinfoDependency.RequestedBy, pathToRoot)
	return nil
}

func (yc *YarnCommand) saveDependenciesData() error {
	log.Debug("Saving data...")

	// Convert map to slice
	var dependenciesSlice, missingDependencies []buildinfo.Dependency
	for _, dependency := range yc.dependencies {
		if dependency.Checksum != nil {
			dependenciesSlice = append(dependenciesSlice, *dependency)
		} else {
			missingDependencies = append(missingDependencies, *dependency)
		}
	}

	if yc.buildConfiguration.Module == "" {
		yc.buildConfiguration.Module = yc.packageInfo.BuildInfoModuleId()
	}

	if err := commandUtils.SaveDependenciesData(dependenciesSlice, yc.buildConfiguration); err != nil {
		return err
	}

	commandUtils.PrintMissingDependencies(missingDependencies)
	return nil
}

type YarnDependency struct {
	// The value is usually in this structure: @scope/package-name@npm:1.0.0
	Value   string         `json:"value,omitempty"`
	Details YarnDepDetails `json:"children,omitempty"`
}

func (yd *YarnDependency) Name() string {
	// Find the first index of '@', starting from position 1. In scoped dependencies (like '@jfrog/package-name@npm:1.2.3') we want to keep the first '@' as part of the name.
	atSignIndex := strings.Index(yd.Value[1:], "@") + 1
	return yd.Value[:atSignIndex]
}

type YarnDepDetails struct {
	Version      string                  `json:"Version,omitempty"`
	Dependencies []YarnDependencyPointer `json:"Dependencies,omitempty"`
}

type YarnDependencyPointer struct {
	Descriptor string `json:"descriptor,omitempty"`
	Locator    string `json:"locator,omitempty"`
}

func createRestoreErrorPrefix(workingDirectory string) string {
	return fmt.Sprintf("Error occurred while restoring the project's %s file. "+
		"To restore the project: delete %s and change the name of the backup file at %s (if exists) to '%s'.\nFailure cause: ",
		yarnrcFileName,
		filepath.Join(workingDirectory, yarnrcFileName),
		filepath.Join(workingDirectory, yarnrcBackupFileName),
		yarnrcFileName)
}

// npmAuth we get back from Artifactory includes several fields, but we need only the field '_auth'
func extractAuthIdentFromNpmAuth(npmAuth string) (string, error) {
	authIdentFieldName := "_auth"
	scanner := bufio.NewScanner(strings.NewReader(npmAuth))

	for scanner.Scan() {
		currLine := scanner.Text()
		if !strings.HasPrefix(currLine, authIdentFieldName) {
			continue
		}

		lineParts := strings.SplitN(currLine, "=", 2)
		if len(lineParts) < 2 {
			return "", errorutils.CheckErrorf("failed while retrieving npm auth details from Artifactory")
		}
		return strings.TrimSpace(lineParts[1]), nil
	}

	return "", errorutils.CheckErrorf("failed while retrieving npm auth details from Artifactory")
}

// Yarn dependency locator usually looks like this: package-name@npm:1.2.3, which is used as the key in the dependencies map.
// But sometimes it points to a virtual package, so it looks different: package-name@virtual:[ID of virtual package]#npm:1.2.3.
// In this case we need to omit the part of the virtual package ID, to get the key as it is found in the dependencies map.
func getYarnDependencyKeyFromLocator(yarnDepLocator string) string {
	virutalIndex := strings.Index(yarnDepLocator, "@virtual:")
	if virutalIndex == -1 {
		return yarnDepLocator
	}

	hashSignIndex := strings.LastIndex(yarnDepLocator, "#")
	return yarnDepLocator[:virutalIndex+1] + yarnDepLocator[hashSignIndex+1:]
}
