package general

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"net/url"
	"strings"
)

// Deduce the server ID from the URL and add server details to config.
func ConfigServerWithDeducedId(server *config.ServerDetails, interactive, webLogin bool) error {
	u, err := url.Parse(server.Url)
	if errorutils.CheckError(err) != nil {
		return err
	}
	// Take the server name from host name: https://myjfrog.jfrog.com/ -> myjfrog
	serverId := strings.Split(u.Host, ".")[0]
	return ConfigServerAsDefault(server, serverId, interactive, webLogin)
}

// Add the given server details to the CLI's config by running a 'jf config' command, and make it the default server.
func ConfigServerAsDefault(server *config.ServerDetails, serverId string, interactive, webLogin bool) error {
	return commands.NewConfigCommand(commands.AddOrEdit, serverId).
		SetInteractive(interactive).SetUseWebLogin(webLogin).
		SetDetails(server).SetMakeDefault(true).Run()
}
