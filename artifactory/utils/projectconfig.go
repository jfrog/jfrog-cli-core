package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
	"path/filepath"
	"reflect"
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
	Go ProjectType = iota
	Pip
	Pipenv
	Poetry
	Npm
	Yarn
	Nuget
	Maven
	Gradle
	Dotnet
	Build
	Terraform
)

// Associates a technology with another of a different type in the structure.
// Docker is not present, as there is no docker-config command and, consequently, no docker.yaml file we need to operate on.
var techType = map[coreutils.Technology]ProjectType{coreutils.Maven: Maven, coreutils.Gradle: Gradle, coreutils.Npm: Npm, coreutils.Yarn: Yarn, coreutils.Go: Go, coreutils.Pip: Pip,
	coreutils.Pipenv: Pipenv, coreutils.Poetry: Poetry, coreutils.Nuget: Nuget, coreutils.Dotnet: Dotnet}

var ProjectTypes = []string{
	"go",
	"pip",
	"pipenv",
	"poetry",
	"npm",
	"yarn",
	"nuget",
	"maven",
	"gradle",
	"dotnet",
	"build",
	"terraform",
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
	NugetV2          bool   `yaml:"nugetV2,omitempty"`
	IncludePatterns  string `yaml:"includePatterns,omitempty"`
	ExcludePatterns  string `yaml:"excludePatterns,omitempty"`
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
			err = fmt.Errorf("%s\nPlease run 'jf %s-config' with your %s repository information",
				err.Error(),
				vConfig.GetString("type"),
				prefix,
			)
		}
	}()
	if !vConfig.IsSet(prefix) {
		err = errorutils.CheckErrorf("the %s repository is missing from the config file (%s)", prefix, configFilePath)
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

func ReadResolutionOnlyConfiguration(confFilePath string) (*RepositoryConfig, error) {
	log.Debug("Preparing to read the config file", confFilePath)
	vConfig, err := ReadConfigFile(confFilePath, YAML)
	if err != nil {
		return nil, err
	}
	return GetRepoConfigByPrefix(confFilePath, ProjectConfigResolverPrefix, vConfig)
}

// Verifies the existence of depsRepo. If it doesn't exist, it searches for a configuration file based on the technology type. If found, it assigns depsRepo in the AuditParams.
func SetResolutionRepoIfExists(params xrayutils.AuditParams, tech coreutils.Technology) (err error) {
	if params.DepsRepo() != "" || params.IgnoreConfigFile() {
		return
	}

	configFilePath, exists, err := GetProjectConfFilePath(techType[tech])
	if err != nil {
		err = fmt.Errorf("failed while searching for %s.yaml config file: %s", tech.String(), err.Error())
		return
	}
	if !exists {
		// Nuget and Dotnet are identified similarly in the detection process. To prevent redundancy, Dotnet is filtered out earlier in the process, focusing solely on detecting Nuget.
		// Consequently, it becomes necessary to verify the presence of dotnet.yaml when Nuget detection occurs.
		if tech == coreutils.Nuget {
			configFilePath, exists, err = GetProjectConfFilePath(techType[coreutils.Dotnet])
			if err != nil {
				err = fmt.Errorf("failed while searching for %s.yaml config file: %s", tech.String(), err.Error())
				return
			}
			if !exists {
				log.Debug(fmt.Sprintf("No %s.yaml nor %s.yaml configuration file was found. Resolving dependencies from %s default registry", coreutils.Nuget.String(), coreutils.Dotnet.String(), tech.String()))
				return
			}
		} else {
			log.Debug(fmt.Sprintf("No %s.yaml configuration file was found. Resolving dependencies from %s default registry", tech.String(), tech.String()))
			return
		}
	}

	log.Debug("Using resolver config from", configFilePath)
	repoConfig, err := ReadResolutionOnlyConfiguration(configFilePath)
	if err != nil {
		err = fmt.Errorf("failed while reading %s.yaml config file: %s", tech.String(), err.Error())
		return
	}
	params.SetServerDetails(repoConfig.serverDetails)
	params.SetDepsRepo(repoConfig.targetRepo)
	return
}
