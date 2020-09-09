package plugins

import (
	"github.com/codegangsta/cli"
	jfrogclicore "github.com/jfrog/jfrog-cli-core"
	"github.com/jfrog/jfrog-cli-core/docs/common"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/log"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

func JfrogPluginMain(jfrogApp App) {
	log.SetDefaultLogger()

	// Set the plugin's user-agent as the jfrog-cli-core's.
	utils.SetUserAgent(jfrogclicore.GetUserAgent())

	cli.CommandHelpTemplate = common.CommandHelpTemplate
	cli.AppHelpTemplate = common.AppHelpTemplate

	baseApp := jfrogApp.convert()
	addHiddenPluginSignatureCommand(baseApp)

	args := os.Args
	err := baseApp.Run(args)

	if cleanupErr := fileutils.CleanOldDirs(); cleanupErr != nil {
		clientLog.Warn(cleanupErr)
	}

	coreutils.ExitOnErr(err)
}
