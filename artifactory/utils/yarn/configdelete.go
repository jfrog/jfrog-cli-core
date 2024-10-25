package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func ConfigDelete(key, executablePath string) error {
	configGetCmdConfig := createConfigDeleteCmdConfig(executablePath, key)
	_, err := gofrogcmd.RunCmdOutput(configGetCmdConfig)
	return errorutils.CheckError(err)
}

func createConfigDeleteCmdConfig(executablePath, key string) *YarnConfig {
	return &YarnConfig{
		Executable: executablePath,
		Command:    []string{"config", "delete", key},
		StrWriter:  nil,
		ErrWriter:  nil,
	}
}
