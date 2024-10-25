package npm

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/repository"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type NpmLoginCommand struct {
	commandName string
	CommonArgs
}

func NewNpmLoginCommand() *NpmLoginCommand {
	return &NpmLoginCommand{commandName: "rt_npm_login"}
}

// Run configures npm to use the specified or selected JFrog Artifactory repository
// for package management, setting up registry and authentication.
func (nlc *NpmLoginCommand) Run() (err error) {
	// If no repository is specified, prompt the user to select an npm-compatible repository.
	if nlc.repo == "" {
		// Define filter parameters to select virtual repositories of npm package type.
		repoFilterParams := services.RepositoriesFilterParams{
			RepoType:    utils.Virtual.String(),
			PackageType: repository.Npm,
		}

		// Select repository interactively based on filter parameters and server details.
		nlc.repo, err = utils.SelectRepositoryInteractively(nlc.serverDetails, repoFilterParams)
		if err != nil {
			return err
		}
	}

	// Initialize NpmrcYarnrcManager for npm to manage registry and authentication configurations.
	npmrcManager := cmdutils.NewNpmrcYarnrcManager(project.Npm, nlc.repo, nlc.serverDetails)

	// Configure the registry URL for npm in the npm configuration.
	if err = npmrcManager.ConfigureRegistry(); err != nil {
		return err
	}

	// Configure authentication settings, handling token or basic auth as needed.
	if err = npmrcManager.ConfigureAuth(); err != nil {
		return err
	}

	// Output success message indicating successful npm configuration.
	log.Output(coreutils.PrintTitle("Successfully configured npm client to work with your JFrog Artifactory repository: " + nlc.repo))
	return nil
}

func (nlc *NpmLoginCommand) CommandName() string {
	return nlc.commandName
}

func (nlc *NpmLoginCommand) ServerDetails() (*config.ServerDetails, error) {
	return nlc.serverDetails, nil
}
