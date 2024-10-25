package npm

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func ConfigDelete(key, executablePath string) error {
	configGetCmdConfig := createConfigDeleteCmdConfig(executablePath, key)
	_, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func createConfigDeleteCmdConfig(executablePath, key string) *npmutils.NpmConfig {
	return &npmutils.NpmConfig{
		Npm:       executablePath,
		Command:   []string{"config", "delete", key},
		StrWriter: nil,
		ErrWriter: nil,
	}
}
