package plugins

import (
	"encoding/json"
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/plugins/components"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const SignatureCommandName = "hidden-plugin-signature"

// Adds a hidden command to every built plugin.
// The command will later be used by the CLI to retrieve the plugin's signature to show in the CLI's help command.
func addHiddenPluginSignatureCommand(baseApp *cli.App) {
	cmd := cli.Command{
		Name:     SignatureCommandName,
		Hidden:   true,
		HideHelp: true,
		Action: func(c *cli.Context) error {
			signature := components.PluginSignature{
				Name:  baseApp.Name,
				Usage: baseApp.Usage,
			}
			content, err := json.Marshal(signature)
			if err == nil {
				log.Output(clientutils.IndentJson(content))
			}
			return err
		},
	}
	baseApp.Commands = append(baseApp.Commands, cmd)
}
