package general

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"net"
	"net/url"
	"strings"
)

const defaultServerId = "default-server"

// Deduce the server ID from the URL and add server details to config.
func ConfigServerWithDeducedId(server *config.ServerDetails, interactive, webLogin bool) error {
	serverId, err := deduceServerId(server.Url)
	if err != nil {
		return err
	}
	return ConfigServerAsDefault(server, serverId, interactive, webLogin)
}

func deduceServerId(platformUrl string) (string, error) {
	u, err := url.Parse(platformUrl)
	if errorutils.CheckError(err) != nil {
		return "", err
	}

	// If the host is an IP address, use a default server ID.
	serverId := defaultServerId
	if net.ParseIP(u.Hostname()) == nil {
		// Otherwise, take the server name from host name: https://myjfrog.jfrog.com/ -> myjfrog
		serverId = strings.Split(u.Hostname(), ".")[0]
	}
	return serverId, nil
}

// Add the given server details to the CLI's config by running a 'jf config' command, and make it the default server.
func ConfigServerAsDefault(server *config.ServerDetails, serverId string, interactive, webLogin bool) error {
	return commands.NewConfigCommand(commands.AddOrEdit, serverId).
		SetInteractive(interactive).SetUseWebLogin(webLogin).
		SetDetails(server).SetMakeDefault(true).Run()
}
