package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

func Info(executablePath string) (string, error) {
	infoCmdConfig := createInfoCmdConfig(executablePath)
	output, err := gofrogcmd.RunCmdOutput(infoCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	infoValue := strings.TrimSpace(output)

	return infoValue, nil
}

func createInfoCmdConfig(executablePath string) *YarnConfig {
	return &YarnConfig{
		Executable: executablePath,
		Command:    []string{"info", "--all", "--recursive", "--json"},
		StrWriter:  nil,
		ErrWriter:  nil,
	}
}
