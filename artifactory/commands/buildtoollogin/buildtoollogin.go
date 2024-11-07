package buildtoollogin

import (
	"fmt"
	biutils "github.com/jfrog/build-info-go/utils"
	pythoncommands "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/repository"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// BuildToolLoginCommand configures registries and authentication for various build tools (npm, Yarn, Pip, Pipenv, Poetry, Go)
// based on the specified project type.
type BuildToolLoginCommand struct {
	// buildTool represents the type of project (e.g., NPM, Yarn).
	buildTool project.ProjectType
	// repoName is the name of the repository used for configuration.
	repoName string
	// serverDetails contains Artifactory server configuration.
	serverDetails *config.ServerDetails
	// commandName specifies the command for this instance.
	commandName string
}

// NewBuildToolLoginCommand initializes a new BuildToolLoginCommand for the specified project type
// and automatically sets a command name for the login operation.
func NewBuildToolLoginCommand(buildTool project.ProjectType) *BuildToolLoginCommand {
	return &BuildToolLoginCommand{
		buildTool:   buildTool,
		commandName: buildTool.String() + "_login",
	}
}

// buildToolToPackageType maps project types to corresponding Artifactory package types (e.g., npm, pypi).
func buildToolToPackageType(buildTool project.ProjectType) (string, error) {
	switch buildTool {
	case project.Npm, project.Yarn:
		return repository.Npm, nil
	case project.Pip, project.Pipenv, project.Poetry:
		return repository.Pypi, nil
	case project.Go:
		return repository.Go, nil
	default:
		return "", errorutils.CheckError(fmt.Errorf("unsupported build tool: %s", buildTool))
	}
}

// CommandName returns the name of the login command.
func (btlc *BuildToolLoginCommand) CommandName() string {
	return btlc.commandName
}

// SetServerDetails assigns the server configuration details to the command.
func (btlc *BuildToolLoginCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildToolLoginCommand {
	btlc.serverDetails = serverDetails
	return btlc
}

// ServerDetails returns the stored server configuration details.
func (btlc *BuildToolLoginCommand) ServerDetails() (*config.ServerDetails, error) {
	return btlc.serverDetails, nil
}

// Run executes the configuration method corresponding to the project type specified for the command.
func (btlc *BuildToolLoginCommand) Run() (err error) {
	if btlc.repoName == "" {
		// Prompt the user to select a virtual repository that matches the project type.
		if err = btlc.GetRepositoryNameFromUserInteractively(); err != nil {
			return err
		}
	}

	// Configure the appropriate tool based on the project type.
	switch btlc.buildTool {
	case project.Npm:
		err = btlc.configureNpm()
	case project.Yarn:
		err = btlc.configureYarn()
	case project.Pip, project.Pipenv:
		err = btlc.configurePip()
	case project.Poetry:
		err = btlc.configurePoetry()
	case project.Go:
		err = btlc.configureGo()
	default:
		err = errorutils.CheckErrorf("unsupported build tool: %s", btlc.buildTool)
	}
	if err != nil {
		return fmt.Errorf("failed to configure %s: %w", btlc.buildTool.String(), err)
	}

	log.Info(fmt.Sprintf("Successfully configured %s to use JFrog Artifactory repository '%s'.", btlc.buildTool.String(), btlc.repoName))
	return nil
}

// GetRepositoryNameFromUserInteractively prompts the user to select a compatible virtual repository.
func (btlc *BuildToolLoginCommand) GetRepositoryNameFromUserInteractively() error {
	// Map the build tool to its corresponding package type.
	packageType, err := buildToolToPackageType(btlc.buildTool)
	if err != nil {
		return err
	}
	repoFilterParams := services.RepositoriesFilterParams{
		RepoType:    utils.Virtual.String(),
		PackageType: packageType,
	}

	// Prompt for repository selection based on filter parameters.
	btlc.repoName, err = utils.SelectRepositoryInteractively(btlc.serverDetails, repoFilterParams)
	return err
}

// configurePip sets the global index-url for pip and pipenv to use the Artifactory PyPI repository.
// Runs the following command:
//
//	pip config set global.index-url https://<user>:<token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
func (btlc *BuildToolLoginCommand) configurePip() error {
	repoWithCredsUrl, err := pythoncommands.GetPypiRepoUrl(btlc.serverDetails, btlc.repoName, false)
	if err != nil {
		return err
	}
	return pythoncommands.RunConfigCommand(project.Pip, []string{"set", "global.index-url", repoWithCredsUrl})
}

// configurePoetry configures Poetry to use the specified repository and authentication credentials.
// Runs the following commands:
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

// configureNpm configures npm to use the Artifactory repository URL and sets authentication.
// Runs the following commands:
//
//	npm config set registry https://<your-artifactory-url>/artifactory/api/npm/<repo-name>
//
// For token-based auth:
//
//	npm config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_authToken "<token>"
//
// For basic auth:
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

// configureYarn configures Yarn to use the specified Artifactory repository and sets authentication.
// Runs the following commands:
//
//	yarn config set registry https://<your-artifactory-url>/artifactory/api/npm/<repo-name>
//
// For token-based auth:
//
//	yarn config set //your-artifactory-url/artifactory/api/npm/<repo-name>/:_authToken "<token>"
//
// For basic auth:
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

// configureGo configures Go to use the Artifactory repository for GOPROXY.
// Runs the following command:
//
//	go env -w GOPROXY=https://<user>:<token>@<your-artifactory-url>/artifactory/go/<repo-name>,direct
func (btlc *BuildToolLoginCommand) configureGo() error {
	repoWithCredsUrl, err := goutils.GetArtifactoryRemoteRepoUrl(btlc.serverDetails, btlc.repoName, goutils.GoProxyUrlParams{Direct: true})
	if err != nil {
		return err
	}
	return biutils.RunGo([]string{"env", "-w", "GOPROXY=" + repoWithCredsUrl}, "")
}
