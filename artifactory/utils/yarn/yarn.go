package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func RunCustomCmd(args []string, executablePath string) error {
	customCmdConfig := createCustomCmdConfig(executablePath, args)
	if err := gofrogcmd.RunCmd(customCmdConfig); err != nil {
		return errorutils.CheckError(err)
	}

	return nil
}

func createCustomCmdConfig(executablePath string, args []string) *YarnConfig {
	return &YarnConfig{
		Executable: executablePath,
		Command:    args,
		StrWriter:  nil,
		ErrWriter:  nil,
	}
}
