package npm

import (
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func ConfigGet(npmFlags []string, confName, executablePath string) (string, error) {
	configGetCmdConfig := createConfigGetCmdConfig(executablePath, confName, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	confValue := strings.TrimSpace(output)

	return confValue, nil
}

func createConfigGetCmdConfig(executablePath, confName string, splitFlags []string) *NpmConfig {
	return &NpmConfig{
		Npm:          executablePath,
		Command:      []string{"config", "get", confName},
		CommandFlags: splitFlags,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
