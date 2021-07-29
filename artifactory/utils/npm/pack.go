package npm

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

func Pack(npmFlags []string, executablePath string) (string, error) {
	packCmd := createPackCmdConfig(executablePath, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(packCmd)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return getPackageFileNameFromOutput(output)
}

func createPackCmdConfig(executablePath string, splitFlags []string) *NpmConfig {
	return &NpmConfig{
		Npm:          executablePath,
		Command:      []string{"pack"},
		CommandFlags: append(splitFlags),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}

func getPackageFileNameFromOutput(output string) (string, error) {
	output = strings.TrimSpace(output)
	lines := strings.Split(output, "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}
