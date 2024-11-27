package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// This method runs "yarn config set" command and sets the yarn configuration.
func ConfigSet(key, value, executablePath string, jsonInput bool) error {
	var flags []string = nil
	if jsonInput {
		flags = append(flags, "--json")
	}
	configGetCmdConfig := createConfigSetCmdConfig(executablePath, key, value, flags)
	_, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	return errorutils.CheckError(err)
}

func createConfigSetCmdConfig(executablePath, key, value string, flags []string) *YarnConfig {
	return &YarnConfig{
		Executable:   executablePath,
		Command:      []string{"config", "set", key, value},
		CommandFlags: flags,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
