package plugins

import (
	"os"

	jfrogclicore "github.com/jfrog/jfrog-cli-core/v2"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
)

const commandHelpTemplate = `{{.HelpName}}{{if .UsageText}}
Arguments:
{{.UsageText}}
{{end}}{{if .VisibleFlags}}
Options:
	{{range .VisibleFlags}}{{.}}
	{{end}}{{end}}{{if .ArgsUsage}}
Environment Variables:
{{.ArgsUsage}}{{end}}

`

const appHelpTemplate = `NAME:
   {{.Name}} - {{.Description}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} [arguments...]{{end}}
   {{if .Version}}
VERSION:
   {{.Version}}
   {{end}}{{if len .Authors}}
AUTHOR(S):
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .VisibleCommands}}
COMMANDS:
   {{range .VisibleCommands}}{{join .Names ", "}}{{ "\t" }}{{if .Description}}{{.Description}}{{else}}{{.Usage}}{{end}}
   {{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
{{end}}

`

func PluginMain(jfrogApp components.App) {
	coreutils.ExitOnErr(RunCliWithPlugin(jfrogApp)())
}

// Use os.Args to pass the command name and arguments to the plugin before running the function to run a specific command.
func RunCliWithPlugin(jfrogApp components.App) func() error {
	return func() error {
		log.SetDefaultLogger()

		// Set the plugin's user-agent as the jfrog-cli-core's.
		utils.SetUserAgent(jfrogclicore.GetUserAgent())

		cli.CommandHelpTemplate = commandHelpTemplate
		cli.AppHelpTemplate = appHelpTemplate

		baseApp, err := components.ConvertApp(jfrogApp)
		if err != nil {
			coreutils.ExitOnErr(err)
		}
		addHiddenPluginSignatureCommand(baseApp)

		args := os.Args
		err = baseApp.Run(args)

		if cleanupErr := fileutils.CleanOldDirs(); cleanupErr != nil {
			clientLog.Warn(cleanupErr)
		}

		return err
	}
}
