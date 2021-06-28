package gradle

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	commandsutils "github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const gradleExtractorDependencyVersion = "4.24.5"

const gradleInitScriptTemplate = "gradle.init"

const usePlugin = "useplugin"
const useWrapper = "usewrapper"
const gradleBuildInfoProperties = "BUILDINFO_PROPFILE"

type GradleCommand struct {
	tasks           string
	configPath      string
	configuration   *utils.BuildConfiguration
	serverDetails   *config.ServerDetails
	threads         int
	detailedSummary bool
	result          *commandsutils.Result
}

func NewGradleCommand() *GradleCommand {
	return &GradleCommand{}
}

// Returns the ArtfiactoryDetails. The information returns from the config file provided.
func (gc *GradleCommand) ServerDetails() (*config.ServerDetails, error) {
	// Get the serverDetails from the config file.
	var err error
	if gc.serverDetails == nil {
		vConfig, err := utils.ReadConfigFile(gc.configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
		gc.serverDetails, err = utils.GetServerDetails(vConfig)
	}
	return gc.serverDetails, err
}

func (gc *GradleCommand) SetServerDetails(serverDetails *config.ServerDetails) *GradleCommand {
	gc.serverDetails = serverDetails
	return gc
}

func (gc *GradleCommand) Run() error {
	gradleDependenciesDir, gradlePluginFilename, err := downloadGradleDependencies()
	if err != nil {
		return err
	}
	gradleRunConfig, err := createGradleRunConfig(gc.tasks, gc.configPath, gc.configuration, gc.threads, gradleDependenciesDir, gradlePluginFilename, gc.detailedSummary)
	if err != nil {
		return err
	}
	defer os.Remove(gradleRunConfig.env[gradleBuildInfoProperties])
	if err := gofrogcmd.RunCmd(gradleRunConfig); err != nil {
		return err
	}
	if gc.IsDetailedSummary() {
		return gc.unmarshalDeployableArtifacts(gradleRunConfig.env[utils.DEPLOYABLE_ARTIFACTS])
	}
	return nil
}

func (gc *GradleCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath)
	if err != nil {
		return err
	}
	gc.SetResult(result)
	return nil
}

func (gc *GradleCommand) CommandName() string {
	return "rt_gradle"
}

func (gc *GradleCommand) SetConfiguration(configuration *utils.BuildConfiguration) *GradleCommand {
	gc.configuration = configuration
	return gc
}

func (gc *GradleCommand) SetConfigPath(configPath string) *GradleCommand {
	gc.configPath = configPath
	return gc
}

func (gc *GradleCommand) SetTasks(tasks string) *GradleCommand {
	gc.tasks = tasks
	return gc
}

func (gc *GradleCommand) SetThreads(threads int) *GradleCommand {
	gc.threads = threads
	return gc
}

func (gc *GradleCommand) SetDetailedSummary(detailedSummary bool) *GradleCommand {
	gc.detailedSummary = detailedSummary
	return gc
}

func (gc *GradleCommand) IsDetailedSummary() bool {
	return gc.detailedSummary
}

func (gc *GradleCommand) Result() *commandsutils.Result {
	return gc.result
}

func (gc *GradleCommand) SetResult(result *commandsutils.Result) *GradleCommand {
	gc.result = result
	return gc
}

func downloadGradleDependencies() (gradleDependenciesDir, gradlePluginFilename string, err error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	gradleDependenciesDir = filepath.Join(dependenciesPath, "gradle", gradleExtractorDependencyVersion)
	gradlePluginFilename = fmt.Sprintf("build-info-extractor-gradle-%s-uber.jar", gradleExtractorDependencyVersion)

	filePath := fmt.Sprintf("org/jfrog/buildinfo/build-info-extractor-gradle/%s", gradleExtractorDependencyVersion)
	downloadPath := path.Join(filePath, gradlePluginFilename)

	filepath.Join(gradleDependenciesDir, gradlePluginFilename)
	err = utils.DownloadExtractorIfNeeded(downloadPath, filepath.Join(gradleDependenciesDir, gradlePluginFilename))
	return
}

func createGradleRunConfig(tasks, configPath string, configuration *utils.BuildConfiguration, threads int, gradleDependenciesDir, gradlePluginFilename string, detailedSummary bool) (*gradleRunConfig, error) {
	runConfig := &gradleRunConfig{env: map[string]string{}}
	runConfig.tasks = tasks

	vConfig, err := utils.ReadConfigFile(configPath, utils.YAML)
	if err != nil {
		return nil, err
	}

	runConfig.gradle, err = getGradleExecPath(vConfig.GetBool(useWrapper))
	if err != nil {
		return nil, err
	}

	if threads > 0 {
		vConfig.Set(utils.FORK_COUNT, threads)
	}

	runConfig.env[gradleBuildInfoProperties], err = utils.CreateBuildInfoPropertiesFile(configuration.BuildName, configuration.BuildNumber, configuration.Project, detailedSummary, vConfig, utils.Gradle)
	if err != nil {
		return nil, err
	}
	if detailedSummary {
		// Save the path to a temp file, where buildinfo project will write the deployable artifacts details.
		runConfig.env[utils.DEPLOYABLE_ARTIFACTS] = vConfig.Get(utils.DEPLOYABLE_ARTIFACTS).(string)
	}

	if !vConfig.GetBool(usePlugin) {
		runConfig.initScript, err = getInitScript(gradleDependenciesDir, gradlePluginFilename)
		if err != nil {
			return nil, err
		}
	}

	return runConfig, nil
}

func getInitScript(gradleDependenciesDir, gradlePluginFilename string) (string, error) {
	gradleDependenciesDir, err := filepath.Abs(gradleDependenciesDir)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	initScriptPath := filepath.Join(gradleDependenciesDir, gradleInitScriptTemplate)

	exists, err := fileutils.IsFileExists(initScriptPath, false)
	if exists || err != nil {
		return initScriptPath, err
	}

	gradlePluginPath := filepath.Join(gradleDependenciesDir, gradlePluginFilename)
	gradlePluginPath = strings.Replace(gradlePluginPath, "\\", "\\\\", -1)
	initScriptContent := strings.Replace(utils.GradleInitScript, "${pluginLibDir}", gradlePluginPath, -1)
	if !fileutils.IsPathExists(gradleDependenciesDir, false) {
		err = os.MkdirAll(gradleDependenciesDir, 0777)
		if errorutils.CheckError(err) != nil {
			return "", err
		}
	}

	return initScriptPath, errorutils.CheckError(ioutil.WriteFile(initScriptPath, []byte(initScriptContent), 0644))
}

type gradleRunConfig struct {
	gradle     string
	tasks      string
	initScript string
	env        map[string]string
}

func (config *gradleRunConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.gradle)
	if config.initScript != "" {
		cmd = append(cmd, "--init-script", config.initScript)
	}
	cmd = append(cmd, strings.Split(config.tasks, " ")...)

	log.Info("Running gradle command:", strings.Join(cmd, " "))
	return exec.Command(cmd[0], cmd[1:]...)
}

func (config *gradleRunConfig) GetEnv() map[string]string {
	return config.env
}

func (config *gradleRunConfig) GetStdWriter() io.WriteCloser {
	return nil
}

func (config *gradleRunConfig) GetErrWriter() io.WriteCloser {
	return nil
}

func getGradleExecPath(useWrapper bool) (string, error) {
	if useWrapper {
		if coreutils.IsWindows() {
			return "gradlew.bat", nil
		}
		return "./gradlew", nil
	}
	gradleExec, err := exec.LookPath("gradle")
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return gradleExec, nil
}
