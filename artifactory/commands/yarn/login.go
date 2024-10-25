package yarn

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

type YarnLoginCommand struct {
	commandName   string
	repo          string
	serverDetails *config.ServerDetails
}

func NewYarnLoginCommand() *YarnLoginCommand {
	return &YarnLoginCommand{commandName: "rt_yarn_login"}
}

// Run configures Yarn to use the specified or selected JFrog Artifactory repository
// for package management, setting up registry and authentication.
func (ylc *YarnLoginCommand) Run() (err error) {
	// If no repository is specified, prompt the user to select an npm-compatible repository.
	if ylc.repo == "" {
		// Define filter parameters to select virtual repositories of npm package type.
		repoFilterParams := services.RepositoriesFilterParams{
			RepoType:    utils.Virtual.String(),
			PackageType: repository.Npm,
		}

		// Select repository interactively based on filter parameters and server details.
		ylc.repo, err = utils.SelectRepositoryInteractively(ylc.serverDetails, repoFilterParams)
		if err != nil {
			return err
		}
	}

	// Initialize NpmrcYarnrcManager for Yarn to manage registry and authentication configurations.
	npmrcManager := cmdutils.NewNpmrcYarnrcManager(project.Yarn, ylc.repo, ylc.serverDetails)

	// Configure the registry URL for Yarn in the Yarn configuration.
	if err = npmrcManager.ConfigureRegistry(); err != nil {
		return err
	}

	// Configure authentication settings, handling token or basic auth as needed.
	if err = npmrcManager.ConfigureAuth(); err != nil {
		return err
	}

	// Output success message indicating successful Yarn configuration.
	log.Output(coreutils.PrintTitle("Successfully configured yarn client to work with your JFrog Artifactory repository: " + ylc.repo))
	return nil
}

func (ylc *YarnLoginCommand) SetServerDetails(serverDetails *config.ServerDetails) *YarnLoginCommand {
	ylc.serverDetails = serverDetails
	return ylc
}

func (ylc *YarnLoginCommand) ServerDetails() (*config.ServerDetails, error) {
	return ylc.serverDetails, nil
}

func (ylc *YarnLoginCommand) CommandName() string {
	return ylc.commandName
}
