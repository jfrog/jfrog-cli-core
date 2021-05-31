package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

func Version(executablePath string) (string, error) {
	versionCmdConfig := createVersionCmdConfig(executablePath)
	output, err := gofrogcmd.RunCmdOutput(versionCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return strings.TrimSpace(output), nil
}

func createVersionCmdConfig(executablePath string) *YarnConfig {
	return &YarnConfig{
		Executable: executablePath,
		Command:    []string{"--version"},
		StrWriter:  nil,
		ErrWriter:  nil,
	}
}
