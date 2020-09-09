package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"path/filepath"
	"reflect"

	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const (
	ProjectConfigResolverPrefix = "resolver"
	ProjectConfigDeployerPrefix = "deployer"
	ProjectConfigRepo           = "repo"
	ProjectConfigServerId       = "serverId"
)

type ProjectType int

const (
	Go ProjectType = iota
	Pip
	Npm
	Nuget
	Maven
	Gradle
	Dotnet
)

var ProjectTypes = []string{
	"go",
	"pip",
	"npm",
	"nuget",
	"maven",
	"gradle",
	"dotnet",
}

func (projectType ProjectType) String() string {
	return ProjectTypes[projectType]
}

type Repository struct {
	Repo             string `yaml:"repo,omitempty"`
	ServerId         string `yaml:"serverId,omitempty"`
	SnapshotRepo     string `yaml:"snapshotRepo,omitempty"`
	ReleaseRepo      string `yaml:"releaseRepo,omitempty"`
	DeployMavenDesc  bool   `yaml:"deployMavenDescriptors,omitempty"`
	DeployIvyDesc    bool   `yaml:"deployIvyDescriptors,omitempty"`
	IvyPattern       string `yaml:"ivyPattern,omitempty"`
	ArtifactsPattern string `yaml:"artifactPattern,omitempty"`
}

type RepositoryConfig struct {
	targetRepo string
	rtDetails  *config.ArtifactoryDetails
}

// If configuration file exists in the working dir or in one of its parent dirs return its path,
// otherwise return the global configuration file path
func GetProjectConfFilePath(projectType ProjectType) (confFilePath string, exists bool, err error) {
	confFileName := filepath.Join("projects", projectType.String()+".yaml")
	projectDir, exists, err := fileutils.FindUpstream(".jfrog", fileutils.Dir)
	if err != nil {
		return "", false, err
	}
	if exists {
		confFilePath = filepath.Join(projectDir, ".jfrog", confFileName)
		exists, err = fileutils.IsFileExists(confFilePath, false)
		if err != nil {
			return "", false, err
		}

		if exists {
			return
		}
	}
	// If missing in the root project, check in the home dir
	jfrogHomeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", exists, err
	}
	confFilePath = filepath.Join(jfrogHomeDir, confFileName)
	exists, err = fileutils.IsFileExists(confFilePath, false)
	return
}

func GetRepoConfigByPrefix(configFilePath, prefix string, vConfig *viper.Viper) (*RepositoryConfig, error) {
	if !vConfig.IsSet(prefix) {
		return nil, errorutils.CheckError(fmt.Errorf("%s information is missing within %s", prefix, configFilePath))
	}
	log.Debug(fmt.Sprintf("Found %s in the config file %s", prefix, configFilePath))
	repo := vConfig.GetString(prefix + "." + ProjectConfigRepo)
	if repo == "" {
		return nil, fmt.Errorf("Missing repository for %s within %s", prefix, configFilePath)
	}
	serverId := vConfig.GetString(prefix + "." + ProjectConfigServerId)
	if serverId == "" {
		return nil, fmt.Errorf("Missing server ID for %s within %s", prefix, configFilePath)
	}
	rtDetails, err := config.GetArtifactorySpecificConfig(serverId, false, true)
	if err != nil {
		return nil, err
	}
	return &RepositoryConfig{targetRepo: repo, rtDetails: rtDetails}, nil
}

func (repo *RepositoryConfig) IsRtDetailsEmpty() bool {
	if repo.rtDetails != nil && reflect.DeepEqual(config.ArtifactoryDetails{}, repo.rtDetails) {
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

func (repo *RepositoryConfig) SetRtDetails(rtDetails *config.ArtifactoryDetails) *RepositoryConfig {
	repo.rtDetails = rtDetails
	return repo
}

func (repo *RepositoryConfig) RtDetails() (*config.ArtifactoryDetails, error) {
	return repo.rtDetails, nil
}

func GetResolutionOnlyConfiguration(projectType ProjectType) (*RepositoryConfig, error) {
	// Get configuration file path.
	confFilePath, exists, err := GetProjectConfFilePath(projectType)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errorutils.CheckError(fmt.Errorf(projectType.String() + " Project configuration does not exist."))
	}
	return ReadResolutionOnlyConfiguration(confFilePath)
}

func ReadResolutionOnlyConfiguration(confFilePath string) (*RepositoryConfig, error) {
	log.Debug("Preparing to read the config file", confFilePath)
	vConfig, err := ReadConfigFile(confFilePath, YAML)
	if err != nil {
		return nil, err
	}
	return GetRepoConfigByPrefix(confFilePath, ProjectConfigResolverPrefix, vConfig)
}
