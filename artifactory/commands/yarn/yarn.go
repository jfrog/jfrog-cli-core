package yarn

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"

	buildinfo "github.com/jfrog/build-info-go/entities"

	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const yarnrcFileName = ".yarnrc.yml"
const yarnrcBackupFileName = "jfrog.yarnrc.backup"
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
	serverDetails      *config.ServerDetails
	authArtDetails     auth.ServiceDetails
	buildConfiguration *utils.BuildConfiguration
	dependencies       map[string]*buildinfo.Dependency
	envVarsBackup      map[string]*string
	buildInfoModule    *build.YarnModule
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
	yc.threads, _, _, _, filteredYarnArgs, yc.buildConfiguration, err = commandUtils.ExtractYarnOptionsFromArgs(yc.yarnArgs)
	if err != nil {
		return err
	}

	if err = yc.preparePrerequisites(); err != nil {
		return err
	}

	var missingDepsChan chan string
	var missingDependencies []string
	if yc.collectBuildInfo {
		missingDepsChan, err = yc.prepareBuildInfo()
		if err != nil {
			return err
		}
		go func() {
			for depId := range missingDepsChan {
				missingDependencies = append(missingDependencies, depId)
			}
		}()
	}

	yc.restoreYarnrcFunc, err = commandUtils.BackupFile(filepath.Join(yc.workingDirectory, yarnrcFileName), filepath.Join(yc.workingDirectory, yarnrcBackupFileName))
	if err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	if err = yc.modifyYarnConfigurations(); err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	yc.buildInfoModule.SetArgs(filteredYarnArgs)
	if err = yc.buildInfoModule.Build(); err != nil {
		return yc.restoreConfigurationsAndError(err)
	}

	if yc.collectBuildInfo {
		close(missingDepsChan)
		commandUtils.PrintMissingDependencies(missingDependencies)
	}

	if err = yc.restoreConfigurationsFromBackup(); err != nil {
		return err
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

	yc.workingDirectory, err = coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	log.Debug("Working directory set to:", yc.workingDirectory)

	yc.collectBuildInfo, err = yc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}

	buildName, err := yc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := yc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}

	buildInfoService := utils.CreateBuildInfoService()
	npmBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, yc.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	yc.buildInfoModule, err = npmBuild.AddYarnModule(yc.workingDirectory)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if yc.buildConfiguration.GetModule() != "" {
		yc.buildInfoModule.SetName(yc.buildConfiguration.GetModule())
	}

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

func (yc *YarnCommand) prepareBuildInfo() (missingDepsChan chan string, err error) {
	log.Info("Preparing for dependencies information collection... For the first run of the build, the dependencies collection may take a few minutes. Subsequent runs should be faster.")
	servicesManager, err := utils.CreateServiceManager(yc.serverDetails, -1, 0, false)
	if err != nil {
		return
	}

	// Collect checksums from last build to decrease requests to Artifactory
	buildName, err := yc.buildConfiguration.GetBuildName()
	if err != nil {
		return
	}
	previousBuildDependencies, err := commandUtils.GetDependenciesFromLatestBuild(servicesManager, buildName)
	if err != nil {
		return
	}
	missingDepsChan = make(chan string)
	collectChecksumsFunc := commandUtils.CreateCollectChecksumsFunc(previousBuildDependencies, servicesManager, missingDepsChan)
	yc.buildInfoModule.SetTraverseDependenciesFunc(collectChecksumsFunc)
	yc.buildInfoModule.SetThreads(yc.threads)
	return
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
