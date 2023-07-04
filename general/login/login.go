package login

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/general"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	newSeverPlaceholder = "New Server"
)

type LoginCommand struct {
}

func NewLoginCommand() *LoginCommand {
	return &LoginCommand{}
}

func (lc *LoginCommand) Run() error {
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	if len(configurations) == 0 {
		return newConfLogin()
	}
	return existingConfLogin(configurations)
}

func newConfLogin() error {
	platformUrl, err := promptPlatformUrl()
	if err != nil {
		return err
	}
	newServer := config.ServerDetails{Url: platformUrl}
	return general.ConfigServerWithDeducedId(&newServer, true, true)
}

func promptPlatformUrl() (string, error) {
	var platformUrl string
	ioutils.ScanFromConsole("JFrog Platform URL", &platformUrl, "")
	if platformUrl == "" {
		return "", errorutils.CheckErrorf("providing JFrog Platform URL is mandatory")
	}
	return platformUrl, nil
}

func existingConfLogin(configurations []*config.ServerDetails) error {
	selectedChoice, err := promptAddOrEdit(configurations)
	if err != nil {
		return err
	}
	if selectedChoice == newSeverPlaceholder {
		return selectedNewServer()
	}
	return existingServerLogin(selectedChoice)
}

// When configurations exist and the user chose to log in with a new server we direct him to a clean config process,
// where he will be prompted for server ID and URL.
func selectedNewServer() error {
	newServer := config.ServerDetails{}
	return general.ConfigServerAsDefault(&newServer, "", true, true)
}

// When a user chose to log in to an existing server,
// we run a config process while keeping all his current server details except credentials.
func existingServerLogin(serverId string) error {
	serverDetails, err := commands.GetConfig(serverId, true)
	if err != nil {
		return err
	}
	serverDetails.User = ""
	serverDetails.Password = ""
	serverDetails.AccessToken = ""
	serverDetails.RefreshToken = ""
	return general.ConfigServerAsDefault(serverDetails, serverId, true, true)
}

// Prompt a list of all server IDs and an option for a new server, and let the user choose to which to log in.
func promptAddOrEdit(configurations []*config.ServerDetails) (selectedChoice string, err error) {
	selectableItems := []ioutils.PromptItem{{Option: newSeverPlaceholder, TargetValue: &selectedChoice}}
	for i := range configurations {
		selectableItems = append(selectableItems, ioutils.PromptItem{Option: configurations[i].ServerId, TargetValue: &selectedChoice})
	}
	err = ioutils.SelectString(selectableItems, "Select whether to create a new server configuration or to web login to an existing one:", false, func(item ioutils.PromptItem) {
		*item.TargetValue = item.Option
		selectedChoice = *item.TargetValue
	})
	return
}
