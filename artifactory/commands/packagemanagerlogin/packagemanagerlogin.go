package packagemanagerlogin

import (
	"fmt"
	bidotnet "github.com/jfrog/build-info-go/build/utils/dotnet"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/dotnet"
	gocommands "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/golang"
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

// PackageManagerLoginCommand configures registries and authentication for various package manager (npm, Yarn, Pip, Pipenv, Poetry, Go)
type PackageManagerLoginCommand struct {
	// packageManager represents the type of package manager (e.g., NPM, Yarn).
	packageManager project.ProjectType
	// repoName is the name of the repository used for configuration.
	repoName string
	// serverDetails contains Artifactory server configuration.
	serverDetails *config.ServerDetails
	// commandName specifies the command for this instance.
	commandName string
}

// NewPackageManagerLoginCommand initializes a new PackageManagerLoginCommand for the specified package manager
// and automatically sets a command name for the login operation.
func NewPackageManagerLoginCommand(packageManager project.ProjectType) *PackageManagerLoginCommand {
	return &PackageManagerLoginCommand{
		packageManager: packageManager,
		commandName:    packageManager.String() + "_login",
	}
}

// packageManagerToPackageType maps project types to corresponding Artifactory package types (e.g., npm, pypi).
func packageManagerToPackageType(packageManager project.ProjectType) (string, error) {
	switch packageManager {
	case project.Npm, project.Yarn:
		return repository.Npm, nil
	case project.Pip, project.Pipenv, project.Poetry:
		return repository.Pypi, nil
	case project.Go:
		return repository.Go, nil
	case project.Nuget, project.Dotnet:
		return repository.Nuget, nil
	default:
		return "", errorutils.CheckErrorf("unsupported package manager: %s", packageManager)
	}
}

// CommandName returns the name of the login command.
func (pmlc *PackageManagerLoginCommand) CommandName() string {
	return pmlc.commandName
}

// SetServerDetails assigns the server configuration details to the command.
func (pmlc *PackageManagerLoginCommand) SetServerDetails(serverDetails *config.ServerDetails) *PackageManagerLoginCommand {
	pmlc.serverDetails = serverDetails
	return pmlc
}

// ServerDetails returns the stored server configuration details.
func (pmlc *PackageManagerLoginCommand) ServerDetails() (*config.ServerDetails, error) {
	return pmlc.serverDetails, nil
}

// Run executes the configuration method corresponding to the package manager specified for the command.
func (pmlc *PackageManagerLoginCommand) Run() (err error) {
	if pmlc.repoName == "" {
		// Prompt the user to select a virtual repository that matches the package manager.
		if err = pmlc.promptUserToSelectRepository(); err != nil {
			return err
		}
	}

	// Configure the appropriate package manager based on the package manager.
	switch pmlc.packageManager {
	case project.Npm:
		err = pmlc.configureNpm()
	case project.Yarn:
		err = pmlc.configureYarn()
	case project.Pip, project.Pipenv:
		err = pmlc.configurePip()
	case project.Poetry:
		err = pmlc.configurePoetry()
	case project.Go:
		err = pmlc.configureGo()
	case project.Nuget, project.Dotnet:
		err = pmlc.configureDotnetNuget()
	default:
		err = errorutils.CheckErrorf("unsupported package manager: %s", pmlc.packageManager)
	}
	if err != nil {
		return fmt.Errorf("failed to configure %s: %w", pmlc.packageManager.String(), err)
	}

	log.Info(fmt.Sprintf("Successfully configured %s to use JFrog Artifactory repository '%s'.", pmlc.packageManager.String(), pmlc.repoName))
	return nil
}

// promptUserToSelectRepository prompts the user to select a compatible virtual repository.
func (pmlc *PackageManagerLoginCommand) promptUserToSelectRepository() error {
	// Map the package manager to its corresponding package type.
	packageType, err := packageManagerToPackageType(pmlc.packageManager)
	if err != nil {
		return err
	}
	repoFilterParams := services.RepositoriesFilterParams{
		RepoType:    utils.Virtual.String(),
		PackageType: packageType,
	}

	// Prompt for repository selection based on filter parameters.
	pmlc.repoName, err = utils.SelectRepositoryInteractively(pmlc.serverDetails, repoFilterParams)
	return err
}

// configurePip sets the global index-url for pip and pipenv to use the Artifactory PyPI repository.
// Runs the following command:
//
//	pip config set global.index-url https://<user>:<token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
func (pmlc *PackageManagerLoginCommand) configurePip() error {
	repoWithCredsUrl, err := pythoncommands.GetPypiRepoUrl(pmlc.serverDetails, pmlc.repoName, false)
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
func (pmlc *PackageManagerLoginCommand) configurePoetry() error {
	repoUrl, username, password, err := pythoncommands.GetPypiRepoUrlWithCredentials(pmlc.serverDetails, pmlc.repoName, false)
	if err != nil {
		return err
	}
	return pythoncommands.RunPoetryConfig(repoUrl.String(), username, password, pmlc.repoName)
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
func (pmlc *PackageManagerLoginCommand) configureNpm() error {
	repoUrl := commandsutils.GetNpmRepositoryUrl(pmlc.repoName, pmlc.serverDetails.ArtifactoryUrl)

	if err := npm.ConfigSet(commandsutils.NpmConfigRegistryKey, repoUrl, "npm"); err != nil {
		return err
	}

	authKey, authValue := commandsutils.GetNpmAuthKeyValue(pmlc.serverDetails, repoUrl)
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
func (pmlc *PackageManagerLoginCommand) configureYarn() error {
	repoUrl := commandsutils.GetNpmRepositoryUrl(pmlc.repoName, pmlc.serverDetails.ArtifactoryUrl)

	if err := yarn.ConfigSet(commandsutils.NpmConfigRegistryKey, repoUrl, "yarn", false); err != nil {
		return err
	}

	authKey, authValue := commandsutils.GetNpmAuthKeyValue(pmlc.serverDetails, repoUrl)
	if authKey != "" && authValue != "" {
		return yarn.ConfigSet(authKey, authValue, "yarn", false)
	}
	return nil
}

// configureGo configures Go to use the Artifactory repository for GOPROXY.
// Runs the following command:
//
//	go env -w GOPROXY=https://<user>:<token>@<your-artifactory-url>/artifactory/go/<repo-name>,direct
func (pmlc *PackageManagerLoginCommand) configureGo() error {
	repoWithCredsUrl, err := gocommands.GetArtifactoryRemoteRepoUrl(pmlc.serverDetails, pmlc.repoName, gocommands.GoProxyUrlParams{Direct: true})
	if err != nil {
		return err
	}
	return biutils.RunGo([]string{"env", "-w", "GOPROXY=" + repoWithCredsUrl}, "")
}

// configureDotnetNuget configures NuGet or .NET Core to use the specified Artifactory repository with credentials.
// Adds the repository source to the NuGet configuration file, using appropriate credentials for authentication.
// The following command is run for dotnet:
//
//	dotnet nuget add source --name <JFrog-Artifactory> "https://acme.jfrog.io/artifactory/api/nuget/{repository-name}" --username <your-username> --password <your-password>
//
// For NuGet:
//
//	nuget sources add -Name <JFrog-Artifactory> -Source "https://acme.jfrog.io/artifactory/api/nuget/{repository-name}" -Username <your-username> -Password <your-password>
func (pmlc *PackageManagerLoginCommand) configureDotnetNuget() error {
	// Retrieve repository URL and credentials for NuGet or .NET Core.
	sourceUrl, user, password, err := dotnet.GetSourceDetails(pmlc.serverDetails, pmlc.repoName, false)
	if err != nil {
		return err
	}

	// Determine the appropriate toolchain type (NuGet or .NET Core).
	toolchainType := bidotnet.DotnetCore
	if pmlc.packageManager == project.Nuget {
		toolchainType = bidotnet.Nuget
	}
	if err = dotnet.RemoveSourceFromNugetConfigIfExists(toolchainType); err != nil {
		return err
	}
	// Add the repository as a source in the NuGet configuration with credentials for authentication.
	return dotnet.AddSourceToNugetConfig(toolchainType, sourceUrl, user, password)
}
