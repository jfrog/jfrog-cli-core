package transferinstall

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

const (
	groovyFileName  = "dataTransfer.groovy"
	jarFileName     = "data-transfer.jar"
	dataTransferUrl = "https://releases.jfrog.io/artifactory/jfrog-releases/data-transfer"
	libDir          = "lib"
)

type InstallTransferCommand struct {
	InstallPluginCommand
}

var transferPluginFiles = PluginFiles{
	PluginFileItem{groovyFileName},
	PluginFileItem{libDir, jarFileName},
}

func NewTransferInstallFileManager() *PluginTransferManager {
	manager := NewArtifactoryPluginTransferManager(transferPluginFiles)
	return manager
}

func (tic *InstallTransferCommand) CommandName() string {
	return "rt_transfer_install"
}

func NewInstallTransferCommand(server *config.ServerDetails) *InstallTransferCommand {
	cmd := &InstallTransferCommand{*NewInstallPluginCommand(server, "Data-Transfer", NewTransferInstallFileManager())}
	cmd.SetBaseDownloadUrl(dataTransferUrl)
	return cmd
}
