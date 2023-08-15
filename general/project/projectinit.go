package project

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	artifactoryCommandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	artifactoryUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"gopkg.in/yaml.v3"
)

const (
	buildFileName = "build.yaml"
)

type ProjectInitCommand struct {
	projectPath string
	serverId    string
	serverUrl   string
}

func NewProjectInitCommand() *ProjectInitCommand {
	return &ProjectInitCommand{}
}

func (pic *ProjectInitCommand) SetProjectPath(path string) *ProjectInitCommand {
	pic.projectPath = path
	return pic
}

func (pic *ProjectInitCommand) SetServerId(id string) *ProjectInitCommand {
	pic.serverId = id
	return pic
}

func (pic *ProjectInitCommand) Run() (err error) {
	if pic.serverId == "" {
		defaultServer, err := config.GetSpecificConfig("", true, false)
		if err != nil {
			return err
		}
		pic.serverId = defaultServer.ServerId
		pic.serverUrl = defaultServer.Url
	}
	technologiesMap, err := pic.detectTechnologies()
	if err != nil {
		return err
	}
	if _, errNotFound := exec.LookPath("docker"); errNotFound == nil {
		technologiesMap[coreutils.Docker] = true
	}
	// First create repositories for the detected technologies.
	for techName := range technologiesMap {
		// First create repositories for the detected technology.
		err = createDefaultReposIfNeeded(techName, pic.serverId)
		if err != nil {
			return err
		}
		err = createProjectBuildConfigs(techName, pic.projectPath, pic.serverId)
		if err != nil {
			return err
		}
	}
	// Create build config
	if err = pic.createBuildConfig(); err != nil {
		return
	}

	err = coreutils.PrintTable("", "", pic.createSummarizeMessage(technologiesMap), false)
	return
}

func (pic *ProjectInitCommand) createSummarizeMessage(technologiesMap map[coreutils.Technology]bool) string {
	return coreutils.PrintBold("This project is initialized!\n") +
		coreutils.PrintBold("The project config is stored inside the .jfrog directory.") +
		"\n\n" +
		coreutils.PrintTitle("🔍 Scan the dependencies of this project for security vulnerabilities by running") +
		"\n" +
		"jf audit\n\n" +
		coreutils.PrintTitle("📦 Scan any software package on you machine for security vulnerabilities by running") +
		"\n" +
		"jf scan path/to/dir/or/package\n\n" +
		coreutils.PrintTitle("🐳 Scan any local docker image on you machine for security vulnerabilities by running") +
		"\n" +
		"jf docker scan <image name>:<image tag>\n\n" +
		coreutils.PrintTitle("💻 If you're using VS Code, IntelliJ IDEA, WebStorm, PyCharm, Android Studio or GoLand") +
		"\n" +
		"Open the IDE 👉 Install the JFrog extension or plugin 👉 View the JFrog panel" +
		"\n\n" +
		pic.createBuildMessage(technologiesMap) +
		coreutils.PrintTitle("📚 Read more using this link:") +
		"\n" +
		coreutils.PrintLink(coreutils.GettingStartedGuideUrl)
}

// Return a string message, which includes all the build and deployment commands, matching the technologiesMap sent.
func (pic *ProjectInitCommand) createBuildMessage(technologiesMap map[coreutils.Technology]bool) string {
	message := ""
	for tech := range technologiesMap {
		switch tech {
		case coreutils.Maven:
			message += "jf mvn install deploy\n"
		case coreutils.Gradle:
			message += "jf gradle artifactoryP\n"
		case coreutils.Npm:
			message += "jf npm install\n"
			message += "jf npm publish\n"
		case coreutils.Go:
			message +=
				"jf go build\n" +
					"jf go-publish v1.0.0\n"
		case coreutils.Pip, coreutils.Pipenv:
			message +=
				"jf " + string(tech) + " install\n" +
					"jf rt upload path/to/package/file default-pypi-local" +
					coreutils.PrintComment(" #Publish your "+string(tech)+" package") +
					"\n"
		case coreutils.Dotnet:
			executableName := coreutils.Nuget
			_, errNotFound := exec.LookPath("dotnet")
			if errNotFound == nil {
				// dotnet exists in path, So use it in the instruction message.
				executableName = coreutils.Dotnet
			}
			message +=
				"jf " + string(executableName) + " restore\n" +
					"jf rt upload '*.nupkg'" + RepoDefaultName[tech][Virtual] + "\n"
		}
	}
	if message != "" {
		message = coreutils.PrintTitle("🚧 Build the code & deploy the packages by running") +
			"\n" +
			message +
			"\n"
	}
	if ok := technologiesMap[coreutils.Docker]; ok {
		baseurl := strings.TrimPrefix(strings.TrimSpace(pic.serverUrl), "https://")
		baseurl = strings.TrimPrefix(baseurl, "http://")
		imageUrl := path.Join(baseurl, DockerVirtualDefaultName, "<image>:<tag>")
		message += coreutils.PrintTitle("🐳 Pull and push any docker image using Artifactory") +
			"\n" +
			"jf docker tag <image>:<tag> " + imageUrl + "\n" +
			"jf docker push " + imageUrl + "\n" +
			"jf docker pull " + imageUrl + "\n\n"
	}

	if message != "" {
		message += coreutils.PrintTitle("📤 Publish the build-info to Artifactory") +
			"\n" +
			"jf rt build-publish\n\n"
	}
	return message
}

// Returns all detected technologies found in the project directory.
// First, try to return only the technologies that detected according to files in the root directory.
// In case no indication found in the root directory, the search continue recursively.
func (pic *ProjectInitCommand) detectTechnologies() (technologiesMap map[coreutils.Technology]bool, err error) {
	technologiesMap, err = coreutils.DetectTechnologies(pic.projectPath, false, false)
	if err != nil {
		return
	}
	// In case no technologies were detected in the root directory, try again recursively.
	if len(technologiesMap) == 0 {
		technologiesMap, err = coreutils.DetectTechnologies(pic.projectPath, false, true)
		if err != nil {
			return
		}
	}
	return
}

type BuildConfigFile struct {
	Version    int    `yaml:"version,omitempty"`
	ConfigType string `yaml:"type,omitempty"`
	BuildName  string `yaml:"name,omitempty"`
}

func (pic *ProjectInitCommand) createBuildConfig() error {
	jfrogProjectDir := filepath.Join(pic.projectPath, ".jfrog", "projects")
	if err := fileutils.CreateDirIfNotExist(jfrogProjectDir); err != nil {
		return errorutils.CheckError(err)
	}
	configFilePath := filepath.Join(jfrogProjectDir, buildFileName)
	projectDirName := filepath.Base(filepath.Dir(pic.projectPath))
	buildConfigFile := &BuildConfigFile{Version: 1, ConfigType: "build", BuildName: projectDirName}
	resBytes, err := yaml.Marshal(&buildConfigFile)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(os.WriteFile(configFilePath, resBytes, 0644))
}

func createDefaultReposIfNeeded(tech coreutils.Technology, serverId string) error {
	err := CreateDefaultLocalRepo(tech, serverId)
	if err != nil {
		return err
	}
	err = CreateDefaultRemoteRepo(tech, serverId)
	if err != nil {
		return err
	}

	return CreateDefaultVirtualRepo(tech, serverId)
}

func createProjectBuildConfigs(tech coreutils.Technology, projectPath string, serverId string) error {
	jfrogProjectDir := filepath.Join(projectPath, ".jfrog", "projects")
	if err := fileutils.CreateDirIfNotExist(jfrogProjectDir); err != nil {
		return errorutils.CheckError(err)
	}
	techName := strings.ToLower(string(tech))
	configFilePath := filepath.Join(jfrogProjectDir, techName+".yaml")
	configFile := artifactoryCommandsUtils.ConfigFile{
		Version:    artifactoryCommandsUtils.BuildConfVersion,
		ConfigType: techName,
	}
	configFile.Resolver = artifactoryUtils.Repository{ServerId: serverId}
	configFile.Deployer = artifactoryUtils.Repository{ServerId: serverId}
	switch tech {
	case coreutils.Maven:
		configFile.Resolver.ReleaseRepo = MavenVirtualDefaultName
		configFile.Resolver.SnapshotRepo = MavenVirtualDefaultName
		configFile.Deployer.ReleaseRepo = MavenVirtualDefaultName
		configFile.Deployer.SnapshotRepo = MavenVirtualDefaultName
	case coreutils.Dotnet:
		fallthrough
	case coreutils.Nuget:
		configFile.Resolver.NugetV2 = true
		fallthrough
	default:
		configFile.Resolver.Repo = RepoDefaultName[tech][Virtual]
		configFile.Deployer.Repo = RepoDefaultName[tech][Virtual]

	}
	resBytes, err := yaml.Marshal(&configFile)
	if err != nil {
		return errorutils.CheckError(err)
	}

	return errorutils.CheckError(os.WriteFile(configFilePath, resBytes, 0644))
}

func (pic *ProjectInitCommand) CommandName() string {
	return "project_init"
}

func (pic *ProjectInitCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetSpecificConfig("", true, false)
}
