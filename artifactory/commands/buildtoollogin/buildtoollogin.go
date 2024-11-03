package buildtoollogin

import (
	"fmt"
	pythoncommands "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/repository"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// BuildToolLoginCommand configures npm, Yarn, Pip, Pipenv, and Poetry registries and authentication
// based on the specified project type.
type BuildToolLoginCommand struct {
	// buildTool represents the project type, either NPM or Yarn.
	buildTool project.ProjectType
	// repoName holds the name of the repository.
	repoName string
	// serverDetails contains configuration details for the Artifactory server.
	serverDetails *config.ServerDetails
	// commandName holds the name of the command.
	commandName string
}

// NewBuildToolLogin initializes a new BuildToolLogin with the given project type,
// repository name, and Artifactory server details.
func NewBuildToolLoginCommand(buildTool project.ProjectType) *BuildToolLoginCommand {
	return &BuildToolLoginCommand{
		buildTool:   buildTool,
		commandName: buildTool.String() + "_login",
	}
}

// Run executes the appropriate configuration method based on the project type.
func (btlc *BuildToolLoginCommand) Run() (err error) {
	// If no repository is specified, prompt the user to select a compatible repository.
	if btlc.repoName == "" {
		if err = btlc.SetVirtualRepoNameInteractively(); err != nil {
			return err
		}
	}

	switch btlc.buildTool {
	case project.Npm:
		err = btlc.configureNpm()
	case project.Yarn:
		err = btlc.configureYarn()
	case project.Pip:
		err = btlc.configurePip()
	case project.Pipenv:
		err = btlc.configurePipenv()
	case project.Poetry:
		err = btlc.configurePoetry()
	default:
		err = errorutils.CheckErrorf("unsupported build tool: %s", btlc.buildTool)
	}
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Successfully configured %s to use JFrog Artifactory repository '%s'.", btlc.buildTool.String(), btlc.repoName))
	return nil
}

// SetVirtualRepoNameInteractively prompts the user to select a virtual repository
func (btlc *BuildToolLoginCommand) SetVirtualRepoNameInteractively() error {
	// Get the package type that corresponds to the build tool.
	packageType, err := buildToolToPackageType(btlc.buildTool)
	if err != nil {
		return err
	}
	// Define filter parameters to select virtual repositories of npm package type.
	repoFilterParams := services.RepositoriesFilterParams{
		RepoType:    utils.Virtual.String(),
		PackageType: packageType,
	}

	// Select repository interactively based on filter parameters and server details.
	btlc.repoName, err = utils.SelectRepositoryInteractively(btlc.serverDetails, repoFilterParams)
	return err
}

// configurePip sets the global index-url for pip to use Artifactory.
// Running the following commands:
//
//	pip config set global index-url https://<user>:<token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
func (btlc *BuildToolLoginCommand) configurePip() error {
	repoWithCredsUrl, err := pythoncommands.GetPypiRepoUrl(btlc.serverDetails, btlc.repoName, false)
	if err != nil {
		return err
	}
	return pythoncommands.RunConfigCommand(btlc.buildTool, []string{"set", "global.index-url", repoWithCredsUrl})
}

// configurePipenv sets the PyPI URL for pipenv to use Artifactory.
// Running the following commands:
//
//	pipenv config set pypi.url https://<user>:<password/token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
func (btlc *BuildToolLoginCommand) configurePipenv() error {
	repoWithCredsUrl, err := pythoncommands.GetPypiRepoUrl(btlc.serverDetails, btlc.repoName, false)
	if err != nil {
		return err
	}
	return pythoncommands.RunConfigCommand(btlc.buildTool, []string{"set", "pypi.url", repoWithCredsUrl})
}

// configurePoetry configures a Poetry repository and basic auth credentials.
// Running the following commands:
//
//	poetry config repositories.<repo-name> https://<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
//	poetry config http-basic.<repo-name> <user> <password/token>
func (btlc *BuildToolLoginCommand) configurePoetry() error {
	repoUrl, username, password, err := pythoncommands.GetPypiRepoUrlWithCredentials(btlc.serverDetails, btlc.repoName, false)
	if err != nil {
		return err
	}
	return pythoncommands.RunPoetryConfig(repoUrl.String(), username, password, btlc.repoName)
}

// configureNpm sets the registry URL and auth for npm to use Artifactory.
// Running the following commands:
//
//	npm config set registry https://<your-artifactory-url>/artifactory/api/npm/<repo-name>
//
// For token-based auth:
//
//	npm config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_authToken "<token>"
//
// For basic auth (username:password):
//
//	npm config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_auth "<base64-encoded-username:password>"
func (btlc *BuildToolLoginCommand) configureNpm() error {
	repoUrl := commandsutils.GetNpmRepositoryUrl(btlc.repoName, btlc.serverDetails.ArtifactoryUrl)

	if err := npm.ConfigSet(commandsutils.NpmConfigRegistryKey, repoUrl, "npm"); err != nil {
		return err
	}

	authKey, authValue := commandsutils.GetNpmAuthKeyValue(btlc.serverDetails, repoUrl)
	if authKey != "" && authValue != "" {
		return npm.ConfigSet(authKey, authValue, "npm")
	}
	return nil
}

// configureYarn sets the registry URL and auth for Yarn to use Artifactory.
// Running the following commands:
//
//	yarn config set registry https://<your-artifactory-url>/artifactory/api/npm/<repo-name>
//
// For token-based auth:
//
//	yarn config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_authToken "<token>"
//
// For basic auth (username:password):
//
//	yarn config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_auth "<base64-encoded-username:password>"
func (btlc *BuildToolLoginCommand) configureYarn() error {
	repoUrl := commandsutils.GetNpmRepositoryUrl(btlc.repoName, btlc.serverDetails.ArtifactoryUrl)

	if err := yarn.ConfigSet(commandsutils.NpmConfigRegistryKey, repoUrl, "yarn", false); err != nil {
		return err
	}

	authKey, authValue := commandsutils.GetNpmAuthKeyValue(btlc.serverDetails, repoUrl)
	if authKey != "" && authValue != "" {
		return yarn.ConfigSet(authKey, authValue, "yarn", false)
	}
	return nil
}

// buildToolToPackageType maps the project type to the corresponding package type.
func buildToolToPackageType(buildTool project.ProjectType) (string, error) {
	switch buildTool {
	case project.Npm, project.Yarn:
		return repository.Npm, nil
	case project.Pip, project.Pipenv, project.Poetry:
		return repository.Pypi, nil
	default:
		return "", errorutils.CheckError(fmt.Errorf("unsupported build tool: %s", buildTool))
	}
}

func (btlc *BuildToolLoginCommand) CommandName() string {
	return btlc.commandName
}

func (btlc *BuildToolLoginCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildToolLoginCommand {
	btlc.serverDetails = serverDetails
	return btlc
}
func (btlc *BuildToolLoginCommand) ServerDetails() (*config.ServerDetails, error) {
	return btlc.serverDetails, nil
}
