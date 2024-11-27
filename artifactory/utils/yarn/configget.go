package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

// This method runs "yarn config set" command and sets the yarn configuration.
func ConfigGet(key, executablePath string, jsonOutput bool) (string, error) {
	var flags []string = nil
	if jsonOutput {
		flags = append(flags, "--json")
	}
	configGetCmdConfig := createConfigGetCmdConfig(executablePath, key, flags)
	output, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	confValue := strings.TrimSpace(output)

	return confValue, nil
}

func createConfigGetCmdConfig(executablePath, confName string, flags []string) *YarnConfig {
	return &YarnConfig{
		Executable:   executablePath,
		Command:      []string{"config", "get", confName},
		CommandFlags: flags,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
