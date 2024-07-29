package yarn

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"

	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	YarnrcFileName       = ".yarnrc.yml"
	YarnrcBackupFileName = "jfrog.yarnrc.backup"
	NpmScopesConfigName  = "npmScopes"
	YarnLockFileName     = "yarn.lock"
	//#nosec G101
	yarnNpmRegistryServerEnv = "YARN_NPM_REGISTRY_SERVER"
	yarnNpmAuthIndent        = "YARN_NPM_AUTH_IDENT"
	// #nosec G101
	yarnNpmAuthToken  = "YARN_NPM_AUTH_TOKEN"
	yarnNpmAlwaysAuth = "YARN_NPM_ALWAYS_AUTH"
)

type YarnCommand struct {
	executablePath     string
	workingDirectory   string
	registry           string
	npmAuthIdent       string
	npmAuthToken       string
	repo               string
	collectBuildInfo   bool
	configFilePath     string
	yarnArgs           []string
	threads            int
	serverDetails      *config.ServerDetails
	buildConfiguration *buildUtils.BuildConfiguration
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

func (yc *YarnCommand) Run() (err error) {
	log.Info("Running Yarn...")
	if err = yc.validateSupportedCommand(); err != nil {
		return
	}

	if err = yc.readConfigFile(); err != nil {
		return
	}

	var filteredYarnArgs []string
	yc.threads, _, _, _, filteredYarnArgs, yc.buildConfiguration, err = commandUtils.ExtractYarnOptionsFromArgs(yc.yarnArgs)
	if err != nil {
		return
	}

	if err = yc.preparePrerequisites(); err != nil {
		return
	}

	var missingDepsChan chan string
	var missingDependencies []string
	if yc.collectBuildInfo {
		missingDepsChan, err = yc.prepareBuildInfo()
		if err != nil {
			return
		}
		go func() {
			for depId := range missingDepsChan {
				missingDependencies = append(missingDependencies, depId)
			}
		}()
	}

	restoreYarnrcFunc, err := ioutils.BackupFile(filepath.Join(yc.workingDirectory, YarnrcFileName), YarnrcBackupFileName)
	if err != nil {
		return errors.Join(err, restoreYarnrcFunc())
	}
	backupEnvMap, err := ModifyYarnConfigurations(yc.executablePath, yc.registry, yc.npmAuthIdent, yc.npmAuthToken)
	if err != nil {
		return errors.Join(err, restoreYarnrcFunc())
	}

	yc.buildInfoModule.SetArgs(filteredYarnArgs)
	if err = yc.buildInfoModule.Build(); err != nil {
		return errors.Join(err, restoreYarnrcFunc())
	}

	if yc.collectBuildInfo {
		close(missingDepsChan)
		commandUtils.PrintMissingDependencies(missingDependencies)
	}

	if err = RestoreConfigurationsFromBackup(backupEnvMap, restoreYarnrcFunc); err != nil {
		return
	}

	log.Info("Yarn finished successfully.")
	return
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
	vConfig, err := project.ReadConfigFile(yc.configFilePath, project.YAML)
	if err != nil {
		return err
	}

	// Extract resolution params
	resolverParams, err := project.GetRepoConfigByPrefix(yc.configFilePath, project.ProjectConfigResolverPrefix, vConfig)
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

	buildInfoService := buildUtils.CreateBuildInfoService()
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

	yc.registry, yc.npmAuthIdent, yc.npmAuthToken, err = GetYarnAuthDetails(yc.serverDetails, yc.repo)
	return err
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

func (yc *YarnCommand) setYarnExecutable() error {
	yarnExecPath, err := exec.LookPath("yarn")
	if err != nil {
		return errorutils.CheckError(err)
	}

	yc.executablePath = yarnExecPath
	log.Debug("Found Yarn executable at:", yc.executablePath)
	return nil
}

func GetYarnAuthDetails(server *config.ServerDetails, repo string) (registry, npmAuthIdent, npmAuthToken string, err error) {
	authRtDetails, err := setArtifactoryAuth(server)
	if err != nil {
		return
	}
	var npmAuthOutput string
	npmAuthOutput, registry, err = commandUtils.GetArtifactoryNpmRepoDetails(repo, authRtDetails, false)
	if err != nil {
		return
	}
	npmAuthIdent, npmAuthToken, err = extractAuthValFromNpmAuth(npmAuthOutput)
	return
}

func setArtifactoryAuth(server *config.ServerDetails) (auth.ServiceDetails, error) {
	authArtDetails, err := server.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	if authArtDetails.GetSshAuthHeaders() != nil {
		return nil, errorutils.CheckErrorf("SSH authentication is not supported in this command")
	}
	return authArtDetails, nil
}

func RestoreConfigurationsFromBackup(envVarsBackup map[string]*string, restoreYarnrcFunc func() error) error {
	if err := restoreEnvironmentVariables(envVarsBackup); err != nil {
		return err
	}
	return restoreYarnrcFunc()
}

func restoreEnvironmentVariables(envVarsBackup map[string]*string) error {
	for key, value := range envVarsBackup {
		if value == nil || *value == "" {
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

func ModifyYarnConfigurations(execPath, registry, npmAuthIdent, npmAuthToken string) (map[string]*string, error) {
	envVarsUpdated := map[string]string{
		yarnNpmRegistryServerEnv: registry,
		yarnNpmAuthIndent:        npmAuthIdent,
		yarnNpmAuthToken:         npmAuthToken,
		yarnNpmAlwaysAuth:        "true",
	}
	envVarsBackup := make(map[string]*string)
	for key, value := range envVarsUpdated {
		oldVal, err := backupAndSetEnvironmentVariable(key, value)
		if err != nil {
			return nil, err
		}
		envVarsBackup[key] = &oldVal
	}
	// Update scoped registries (these cannot be set in environment variables)
	return envVarsBackup, errorutils.CheckError(updateScopeRegistries(execPath, registry, npmAuthIdent, npmAuthToken))
}

func updateScopeRegistries(execPath, registry, npmAuthIdent, npmAuthToken string) error {
	npmScopesStr, err := yarn.ConfigGet(NpmScopesConfigName, execPath, true)
	if err != nil {
		return err
	}
	npmScopesMap := make(map[string]yarnNpmScope)
	err = json.Unmarshal([]byte(npmScopesStr), &npmScopesMap)
	if err != nil {
		return errorutils.CheckError(err)
	}
	artifactoryScope := yarnNpmScope{NpmAlwaysAuth: true, NpmAuthIdent: npmAuthIdent, NpmAuthToken: npmAuthToken, NpmRegistryServer: registry}
	for scopeName := range npmScopesMap {
		npmScopesMap[scopeName] = artifactoryScope
	}
	updatedNpmScopesStr, err := json.Marshal(npmScopesMap)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return yarn.ConfigSet(NpmScopesConfigName, string(updatedNpmScopesStr), execPath, true)
}

type yarnNpmScope struct {
	NpmAlwaysAuth     bool   `json:"npmAlwaysAuth,omitempty"`
	NpmAuthIdent      string `json:"npmAuthIdent,omitempty"`
	NpmAuthToken      string `json:"npmAuthToken,omitempty"`
	NpmRegistryServer string `json:"npmRegistryServer,omitempty"`
}

func backupAndSetEnvironmentVariable(key, value string) (string, error) {
	oldVal, _ := os.LookupEnv(key)
	return oldVal, errorutils.CheckError(os.Setenv(key, value))
}

// npmAuth includes several fields, but we need only the field '_auth' or '_authToken'
func extractAuthValFromNpmAuth(npmAuth string) (authIndent, authToken string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(npmAuth))

	for scanner.Scan() {
		currLine := scanner.Text()
		if !strings.HasPrefix(currLine, commandUtils.NpmConfigAuthKey) {
			continue
		}

		lineParts := strings.SplitN(currLine, "=", 2)
		if len(lineParts) < 2 {
			return "", "", errorutils.CheckErrorf("failed while retrieving npm auth details from Artifactory")
		}
		authVal := strings.TrimSpace(lineParts[1])

		switch strings.TrimSpace(lineParts[0]) {
		case commandUtils.NpmConfigAuthKey:
			return authVal, "", nil
		case commandUtils.NpmConfigAuthTokenKey:
			return "", authVal, nil
		default:
			return "", "", errorutils.CheckErrorf("unexpected auth key found in npm auth")
		}
	}

	return "", "", errorutils.CheckErrorf("failed while retrieving npm auth details from Artifactory")
}
