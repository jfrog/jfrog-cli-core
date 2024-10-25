package npm

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
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

func createConfigSetCmdConfig(executablePath, key, value string) *npmutils.NpmConfig {
	return &npmutils.NpmConfig{
		Npm:       executablePath,
		Command:   []string{"config", "set", key, value},
		StrWriter: nil,
		ErrWriter: nil,
	}
}
