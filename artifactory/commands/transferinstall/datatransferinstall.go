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

type InstallDataTransferPluginCommand struct {
	InstallPluginCommand
}

var transferPluginFiles = PluginFiles{
	FileItem{groovyFileName},
	FileItem{libDir, jarFileName},
}

func NewDataTransferInstallFileManager() *PluginInstallManager {
	manager := NewArtifactoryPluginInstallManager(transferPluginFiles)
	return manager
}

func (tic *InstallDataTransferPluginCommand) CommandName() string {
	return "rt_transfer_install"
}

func NewInstallDataTransferCommand(server *config.ServerDetails) *InstallDataTransferPluginCommand {
	cmd := &InstallDataTransferPluginCommand{*NewInstallPluginCommand(server, "data-transfer", NewDataTransferInstallFileManager())}
	cmd.SetBaseDownloadUrl(dataTransferUrl)
	return cmd
}
