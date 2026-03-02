package commands

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func init() {
	clientlog.SetLogger(clientlog.NewLogger(clientlog.WARN, nil)) // Disable "[Info] *** build config successfully created." messages
}

func TestGoConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath), "Couldn't remove temp dir")
	}()

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Go)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Go.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestGoConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo", deploymentRepo+"=repo-local")
	err = CreateBuildConfig(context, project.Go)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Go.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestPipConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Pip)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Pip.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestPipConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo", deploymentRepo+"=repo-local")
	err = CreateBuildConfig(context, project.Pip)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Pip.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestPipenvConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath))
	}()

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Pipenv)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Pipenv.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestPipenvConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo", deploymentRepo+"=repo-local")
	err = CreateBuildConfig(context, project.Pipenv)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Pipenv.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestNpmConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Npm)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Npm.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestRubyConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Ruby)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Ruby.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestConanConfigFile(t *testing.T) {
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Conan)
	assert.NoError(t, err)

	config := checkCommonAndGetConfiguration(t, project.Conan.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestNpmConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo", deploymentRepo+"=repo-local")
	err = CreateBuildConfig(context, project.Npm)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Npm.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
}

func TestNugetConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo")
	err := CreateBuildConfig(context, project.Nuget)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Nuget.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, true, config.GetBool("resolver.nugetV2"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestNugetConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo")
	err = CreateBuildConfig(context, project.Nuget)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Nuget.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, true, config.GetBool("resolver.nugetV2"))
}

func TestMavenConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionReleasesRepo+"=release-repo", resolutionSnapshotsRepo+"=snapshot-repo",
		deploymentServerId+"=depServer", deploymentReleasesRepo+"=release-repo-local", deploymentSnapshotsRepo+"=snapshot-repo-local",
		includePatterns+"=*pattern*;second", excludePatterns+"=excluding;*pattern")
	err := CreateBuildConfig(context, project.Maven)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Maven.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "snapshot-repo", config.GetString("resolver.snapshotRepo"))
	assert.Equal(t, "release-repo", config.GetString("resolver.releaseRepo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "snapshot-repo-local", config.GetString("deployer.snapshotRepo"))
	assert.Equal(t, "release-repo-local", config.GetString("deployer.releaseRepo"))
	assert.Equal(t, "*pattern*;second", config.GetString("deployer.includePatterns"))
	assert.Equal(t, "excluding;*pattern", config.GetString("deployer.excludePatterns"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestMavenConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionReleasesRepo+"=release-repo", resolutionSnapshotsRepo+"=snapshot-repo",
		deploymentReleasesRepo+"=release-repo-local", deploymentSnapshotsRepo+"=snapshot-repo-local")
	err = CreateBuildConfig(context, project.Maven)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Maven.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "snapshot-repo", config.GetString("resolver.snapshotRepo"))
	assert.Equal(t, "release-repo", config.GetString("resolver.releaseRepo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "snapshot-repo-local", config.GetString("deployer.snapshotRepo"))
	assert.Equal(t, "release-repo-local", config.GetString("deployer.releaseRepo"))
}

func TestGradleConfigFile(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local",
		ivyDescPattern+"=[ivy]/[pattern]", ivyArtifactsPattern+"=[artifact]/[pattern]")
	err := CreateBuildConfig(context, project.Gradle)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Gradle.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
	assert.Equal(t, true, config.GetBool("deployer.deployMavenDescriptors"))
	assert.Equal(t, true, config.GetBool("deployer.deployIvyDescriptors"))
	assert.Equal(t, "[ivy]/[pattern]", config.GetString("deployer.ivyPattern"))
	assert.Equal(t, "[artifact]/[pattern]", config.GetString("deployer.artifactPattern"))
	assert.Equal(t, true, config.GetBool("usePlugin"))
	assert.Equal(t, true, config.GetBool("useWrapper"))
}

// In case resolver/deployer server-id flags are not provided - the default configured global server will be chosen.
func TestGradleConfigFileWithDefaultServerId(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	cleanUp, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUp()

	// Create build config
	context := createContext(t, resolutionRepo+"=repo", deploymentRepo+"=repo-local",
		ivyDescPattern+"=[ivy]/[pattern]", ivyArtifactsPattern+"=[artifact]/[pattern]")
	err = CreateBuildConfig(context, project.Gradle)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Gradle.String(), os.Getenv(coreutils.HomeDir))
	assert.Equal(t, "test", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "test", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
	assert.Equal(t, true, config.GetBool("deployer.deployMavenDescriptors"))
	assert.Equal(t, true, config.GetBool("deployer.deployIvyDescriptors"))
	assert.Equal(t, "[ivy]/[pattern]", config.GetString("deployer.ivyPattern"))
	assert.Equal(t, "[artifact]/[pattern]", config.GetString("deployer.artifactPattern"))
	assert.Equal(t, true, config.GetBool("usePlugin"))
	assert.Equal(t, true, config.GetBool("useWrapper"))
}

func TestGradleConfigFileDefaultPatterns(t *testing.T) {
	// Set JFROG_CLI_HOME_DIR environment variable
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)

	// Create build config
	context := createContext(t, resolutionServerId+"=relServer", resolutionRepo+"=repo", deploymentServerId+"=depServer", deploymentRepo+"=repo-local")
	err := CreateBuildConfig(context, project.Gradle)
	assert.NoError(t, err)

	// Check configuration
	config := checkCommonAndGetConfiguration(t, project.Gradle.String(), tempDirPath)
	assert.Equal(t, "relServer", config.GetString("resolver.serverId"))
	assert.Equal(t, "repo", config.GetString("resolver.repo"))
	assert.Equal(t, "depServer", config.GetString("deployer.serverId"))
	assert.Equal(t, "repo-local", config.GetString("deployer.repo"))
	assert.Equal(t, true, config.GetBool("deployer.deployMavenDescriptors"))
	assert.Equal(t, true, config.GetBool("deployer.deployIvyDescriptors"))
	assert.Equal(t, "[organization]/[module]/ivy-[revision].xml", config.GetString("deployer.ivyPattern"))
	assert.Equal(t, "[organization]/[module]/[revision]/[artifact]-[revision](-[classifier]).[ext]", config.GetString("deployer.artifactPattern"))
	assert.Equal(t, true, config.GetBool("usePlugin"))
	assert.Equal(t, true, config.GetBool("useWrapper"))
}

func TestValidateConfigResolver(t *testing.T) {
	// Create and check empty config
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)
	configFile := NewConfigFile(project.Go, createContext(t))
	err := configFile.validateConfig()
	assert.NoError(t, err)

	// Check scenarios of serverId and repo
	configFile.Resolver.ServerId = "serverId"
	err = configFile.validateConfig()
	assert.EqualError(t, err, resolutionErrorPrefix+setRepositoryError)
	configFile.Resolver.Repo = "repo"
	err = configFile.validateConfig()
	assert.NoError(t, err)

	// Set Server Id with environment variable
	configFile.Resolver.ServerId = ""
	configFile.Resolver.Repo = "repo"
	setEnvCallBack := testsutils.SetEnvWithCallbackAndAssert(t, coreutils.ServerID, "serverId")
	err = configFile.validateConfig()
	assert.NoError(t, err)
	setEnvCallBack()

	configFile.Resolver.ServerId = ""
	err = configFile.validateConfig()
	assert.EqualError(t, err, resolutionErrorPrefix+setServerIdError)

	// Check scenarios of serverId and release/snapshot repositories
	configFile.Resolver.ServerId = "serverId"
	configFile.Resolver.SnapshotRepo = "snapshotRepo"
	err = configFile.validateConfig()
	assert.EqualError(t, err, resolutionErrorPrefix+setSnapshotAndReleaseError)
	configFile.Resolver.ReleaseRepo = "releaseRepo"
	err = configFile.validateConfig()
	assert.NoError(t, err)
	configFile.Resolver.ServerId = ""
	err = configFile.validateConfig()
	assert.EqualError(t, err, resolutionErrorPrefix+setServerIdError)
}

func TestValidateConfigDeployer(t *testing.T) {
	// Create and check empty config
	tempDirPath := createTempEnv(t)
	defer testsutils.RemoveAllAndAssert(t, tempDirPath)
	configFile := NewConfigFile(project.Go, createContext(t))
	err := configFile.validateConfig()
	assert.NoError(t, err)

	// Check scenarios of serverId and repo
	configFile.Deployer.ServerId = "serverId"
	err = configFile.validateConfig()
	assert.EqualError(t, err, deploymentErrorPrefix+setRepositoryError)
	configFile.Deployer.Repo = "repo"
	err = configFile.validateConfig()
	assert.NoError(t, err)

	// Set Server Id with environment variable
	configFile.Deployer.ServerId = ""
	configFile.Deployer.Repo = "repo"
	setEnvCallBack := testsutils.SetEnvWithCallbackAndAssert(t, coreutils.ServerID, "serverId")
	err = configFile.validateConfig()
	assert.NoError(t, err)
	setEnvCallBack()

	configFile.Deployer.ServerId = ""
	err = configFile.validateConfig()
	assert.EqualError(t, err, deploymentErrorPrefix+setServerIdError)

	// Check scenarios of serverId and release/snapshot repositories
	configFile.Deployer.ServerId = "serverId"
	configFile.Deployer.ReleaseRepo = "releaseRepo"
	err = configFile.validateConfig()
	assert.EqualError(t, err, deploymentErrorPrefix+setSnapshotAndReleaseError)
	configFile.Deployer.SnapshotRepo = "snapshotRepo"
	err = configFile.validateConfig()
	assert.NoError(t, err)
	configFile.Deployer.ServerId = ""
	err = configFile.validateConfig()
	assert.EqualError(t, err, deploymentErrorPrefix+setServerIdError)
}

// Set JFROG_CLI_HOME_DIR environment variable to be a new temp directory
func createTempEnv(t *testing.T) string {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	testsutils.SetEnvAndAssert(t, coreutils.HomeDir, tmpDir)
	return tmpDir
}

// Create new Codegangsta context with all required flags.
func createContext(t *testing.T, stringFlags ...string) *cli.Context {
	flagSet := flag.NewFlagSet("TestFlagSet", flag.ContinueOnError)
	flags := setBoolFlags(flagSet, global, usesPlugin, useWrapper, deployMavenDesc, deployIvyDesc, nugetV2)
	flags = append(flags, setStringFlags(flagSet, stringFlags...)...)
	assert.NoError(t, flagSet.Parse(flags))
	return cli.NewContext(nil, flagSet, nil)
}

// Set boolean flags and initialize them to true. Return a slice of them.
func setBoolFlags(flagSet *flag.FlagSet, flags ...string) []string {
	cmdFlags := []string{}
	for _, boolFlag := range flags {
		flagSet.Bool(boolFlag, true, "")
		cmdFlags = append(cmdFlags, "--"+boolFlag)
	}
	return cmdFlags
}

// Set string flags. Return a slice of their values.
func setStringFlags(flagSet *flag.FlagSet, flags ...string) []string {
	cmdFlags := []string{}
	for _, stringFlag := range flags {
		flagSet.String(strings.Split(stringFlag, "=")[0], "", "")
		cmdFlags = append(cmdFlags, "--"+stringFlag)
	}
	return cmdFlags
}

// Read yaml configuration from disk, check version and type.
func checkCommonAndGetConfiguration(t *testing.T, projectType string, tempDirPath string) *viper.Viper {
	config, err := project.ReadConfigFile(filepath.Join(tempDirPath, "projects", projectType+".yaml"), project.YAML)
	assert.NoError(t, err)
	assert.Equal(t, BuildConfVersion, config.GetInt("version"))
	assert.Equal(t, projectType, config.GetString("type"))
	return config
}
