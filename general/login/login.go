package login

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/general"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	newSeverPlaceholder = "[New Server]"
)

type LoginCommand struct {
	serverId            string
	disableTokenRefresh *bool
	// When true, preserve per-service URL prompts (Artifactory/Distribution/Xray/Mission Control/Pipelines)
	// and skip the browser-based web login, for Artifactory v6.x self-hosted customers.
	legacy bool
}

func NewLoginCommand() *LoginCommand {
	return &LoginCommand{}
}

func (lc *LoginCommand) SetServerId(serverId string) *LoginCommand {
	lc.serverId = serverId
	return lc
}

// SetDisableTokenRefresh sets whether automatic access token refresh should be disabled for the logged-in server.
// Pass nil to leave any previously configured value untouched (e.g. when the CLI flag wasn't explicitly provided).
func (lc *LoginCommand) SetDisableTokenRefresh(disableTokenRefresh *bool) *LoginCommand {
	lc.disableTokenRefresh = disableTokenRefresh
	return lc
}

func (lc *LoginCommand) SetLegacy(legacy bool) *LoginCommand {
	lc.legacy = legacy
	return lc
}

func (lc *LoginCommand) Run() error {
	if lc.serverId != "" {
		return existingServerLogin(lc.serverId, lc.disableTokenRefresh, lc.legacy)
	}
	configurations, err := config.GetAllServersConfigs()
	if err != nil {
		return err
	}
	if len(configurations) == 0 {
		return newConfLogin(lc.disableTokenRefresh, lc.legacy)
	}
	return existingConfLogin(configurations, lc.disableTokenRefresh, lc.legacy)
}

func newConfLogin(disableTokenRefresh *bool, legacy bool) error {
	platformUrl := promptPlatformUrl()
	newServer := config.ServerDetails{Url: platformUrl}
	if disableTokenRefresh != nil {
		newServer.DisableTokenRefresh = *disableTokenRefresh
	}
	return general.ConfigServerWithDeducedId(&newServer, true, !legacy, legacy)
}

func promptPlatformUrl() string {
	var platformUrl string
	// Loop until a non-empty platformUrl is entered
	for {
		ioutils.ScanFromConsole("Enter your JFrog Platform URL", &platformUrl, "")
		if platformUrl != "" {
			break
		}
		fmt.Println("The JFrog Platform URL cannot be empty. Please try again.")
	}
	return platformUrl
}

func existingConfLogin(configurations []*config.ServerDetails, disableTokenRefresh *bool, legacy bool) error {
	selectedChoice, err := promptAddOrEdit(configurations)
	if err != nil {
		return err
	}
	if selectedChoice == newSeverPlaceholder {
		return selectedNewServer(disableTokenRefresh, legacy)
	}
	return existingServerLogin(selectedChoice, disableTokenRefresh, legacy)
}

// When configurations exist and the user chose to log in with a new server we direct him to a clean config process,
// where he will be prompted for server ID and URL.
func selectedNewServer(disableTokenRefresh *bool, legacy bool) error {
	var newServer *config.ServerDetails
	if disableTokenRefresh != nil {
		newServer = &config.ServerDetails{DisableTokenRefresh: *disableTokenRefresh}
	}
	return general.ConfigServerAsDefault(newServer, "", true, !legacy, legacy)
}

// When a user chose to log in to an existing server,
// we run a config process while keeping all his current server details except credentials.
func existingServerLogin(serverId string, disableTokenRefresh *bool, legacy bool) error {
	serverDetails, err := commands.GetConfig(serverId, true)
	if err != nil {
		return err
	}
	if serverDetails.Url == "" {
		serverDetails = &config.ServerDetails{ServerId: serverDetails.ServerId}
	} else {
		if !legacy && fileutils.IsSshUrl(serverDetails.Url) {
			return errorutils.CheckErrorf("web login cannot be performed via SSH. Please try again with different server configuration or configure a new one")
		}
		serverDetails.User = ""
		serverDetails.Password = ""
		serverDetails.AccessToken = ""
		serverDetails.RefreshToken = ""
	}
	if disableTokenRefresh != nil {
		serverDetails.DisableTokenRefresh = *disableTokenRefresh
	}
	return general.ConfigServerAsDefault(serverDetails, serverId, true, !legacy, legacy)
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
