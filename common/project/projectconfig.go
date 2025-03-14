package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const (
	ProjectConfigResolverPrefix = "resolver"
	ProjectConfigDeployerPrefix = "deployer"
	ProjectConfigRepo           = "repo"
	ProjectConfigReleaseRepo    = "releaseRepo"
	ProjectConfigServerId       = "serverId"
)

type ProjectType int

const (
	// When adding new ProjectType here, Must also add it as a string to the ProjectTypes slice
	Go ProjectType = iota
	Pip
	Pipenv
	Poetry
	Npm
	Pnpm
	Yarn
	Nuget
	Maven
	Gradle
	Dotnet
	Build
	Terraform
	Cocoapods
	Swift
	Docker
	Podman
)

type ConfigType string

const (
	YAML       ConfigType = "yaml"
	PROPERTIES ConfigType = "properties"
)

var ProjectTypes = []string{
	"go",
	"pip",
	"pipenv",
	"poetry",
	"npm",
	"pnpm",
	"yarn",
	"nuget",
	"maven",
	"gradle",
	"dotnet",
	"build",
	"terraform",
	"cocoapods",
	"swift",
	"docker",
	"podman",
}

func (projectType ProjectType) String() string {
	return ProjectTypes[projectType]
}

// FromString converts a string to its corresponding ProjectType
func FromString(value string) ProjectType {
	for i, projectType := range ProjectTypes {
		if projectType == value {
			return ProjectType(i)
		}
	}
	return -1
}

type MissingResolverErr struct {
	message string
}

func (mre *MissingResolverErr) Error() string {
	return mre.message
}

type Repository struct {
	Repo                  string `yaml:"repo,omitempty"`
	ServerId              string `yaml:"serverId,omitempty"`
	SnapshotRepo          string `yaml:"snapshotRepo,omitempty"`
	DisableSnapshots      bool   `yaml:"disableSnapshots,omitempty"`
	SnapshotsUpdatePolicy string `yaml:"snapshotsUpdatePolicy,omitempty"`
	ReleaseRepo           string `yaml:"releaseRepo,omitempty"`
	DeployMavenDesc       bool   `yaml:"deployMavenDescriptors,omitempty"`
	DeployIvyDesc         bool   `yaml:"deployIvyDescriptors,omitempty"`
	IvyPattern            string `yaml:"ivyPattern,omitempty"`
	ArtifactsPattern      string `yaml:"artifactPattern,omitempty"`
	NugetV2               bool   `yaml:"nugetV2,omitempty"`
	IncludePatterns       string `yaml:"includePatterns,omitempty"`
	ExcludePatterns       string `yaml:"excludePatterns,omitempty"`
}

type RepositoryConfig struct {
	targetRepo    string
	serverDetails *config.ServerDetails
}

// If configuration file exists in the working dir or in one of its parent dirs return its path,
// otherwise return the global configuration file path
func GetProjectConfFilePath(projectType ProjectType) (confFilePath string, exists bool, err error) {
	confFileName := filepath.Join("projects", projectType.String()+".yaml")
	projectDir, exists, err := fileutils.FindUpstream(".jfrog", fileutils.Dir)
	if err != nil {
		return
	}
	if exists {
		filePath := filepath.Join(projectDir, ".jfrog", confFileName)
		exists, err = fileutils.IsFileExists(filePath, false)
		if err != nil {
			return
		}

		if exists {
			confFilePath = filePath
			return
		}
	}
	// If missing in the root project, check in the home dir
	jfrogHomeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return
	}
	filePath := filepath.Join(jfrogHomeDir, confFileName)
	exists, err = fileutils.IsFileExists(filePath, false)
	if exists {
		confFilePath = filePath
	}
	return
}

func GetRepoConfigByPrefix(configFilePath, prefix string, vConfig *viper.Viper) (repoConfig *RepositoryConfig, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(err, fmt.Errorf("please run 'jf %s-config' with your %s repository information", vConfig.GetString("type"), prefix))
		}
	}()
	if !vConfig.IsSet(prefix) {
		err = &MissingResolverErr{fmt.Sprintf("the %s repository is missing from the config file (%s)", prefix, configFilePath)}
		return
	}
	log.Debug(fmt.Sprintf("Found %s in the config file %s", prefix, configFilePath))
	repo := vConfig.GetString(prefix + "." + ProjectConfigRepo)
	if repo == "" {
		// In the maven.yaml config, there's a resolver repository field named "releaseRepo"
		if repo = vConfig.GetString(prefix + "." + ProjectConfigReleaseRepo); repo == "" {
			err = errorutils.CheckErrorf("missing repository for %s within %s", prefix, configFilePath)
			return
		}
	}
	serverId := vConfig.GetString(prefix + "." + ProjectConfigServerId)
	if serverId == "" {
		err = errorutils.CheckErrorf("missing server ID for %s within %s", prefix, configFilePath)
		return
	}
	rtDetails, err := config.GetSpecificConfig(serverId, false, true)
	if err != nil {
		return
	}
	repoConfig = &RepositoryConfig{targetRepo: repo, serverDetails: rtDetails}
	return
}

func (repo *RepositoryConfig) IsServerDetailsEmpty() bool {
	if repo.serverDetails != nil && reflect.DeepEqual(config.ServerDetails{}, repo.serverDetails) {
		return false
	}
	return true
}

func (repo *RepositoryConfig) SetTargetRepo(targetRepo string) *RepositoryConfig {
	repo.targetRepo = targetRepo
	return repo
}

func (repo *RepositoryConfig) TargetRepo() string {
	return repo.targetRepo
}

func (repo *RepositoryConfig) SetServerDetails(rtDetails *config.ServerDetails) *RepositoryConfig {
	repo.serverDetails = rtDetails
	return repo
}

func (repo *RepositoryConfig) ServerDetails() (*config.ServerDetails, error) {
	return repo.serverDetails, nil
}

func GetResolutionOnlyConfiguration(projectType ProjectType) (*RepositoryConfig, error) {
	// Get configuration file path.
	confFilePath, exists, err := GetProjectConfFilePath(projectType)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errorutils.CheckErrorf(projectType.String() + " Project configuration does not exist.")
	}
	return ReadResolutionOnlyConfiguration(confFilePath)
}

func ReadConfigFile(configPath string, configType ConfigType) (config *viper.Viper, err error) {
	config = viper.New()
	config.SetConfigType(string(configType))

	f, err := os.Open(configPath)
	if err != nil {
		return config, errorutils.CheckError(err)
	}
	defer func() {
		err = errorutils.CheckError(f.Close())
	}()
	err = config.ReadConfig(f)
	return config, errorutils.CheckError(err)
}

func ReadResolutionOnlyConfiguration(confFilePath string) (*RepositoryConfig, error) {
	log.Debug("Preparing to read the config file", confFilePath)
	vConfig, err := ReadConfigFile(confFilePath, YAML)
	if err != nil {
		return nil, err
	}
	return GetRepoConfigByPrefix(confFilePath, ProjectConfigResolverPrefix, vConfig)
}
