package npm

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func ConfigSet(key, value, executablePath string) error {
	configGetCmdConfig := createConfigSetCmdConfig(executablePath, key, value)
	_, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func createConfigSetCmdConfig(executablePath, key, value string) *NpmConfig {
	return &NpmConfig{
		Npm:       executablePath,
		Command:   []string{"config", "set", key, value},
		StrWriter: nil,
		ErrWriter: nil,
	}
}
