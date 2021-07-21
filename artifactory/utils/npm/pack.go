package npm

import (
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func Pack(npmFlags []string, executablePath string) (string, error) {

	configListCmdConfig := createPackCmdConfig(executablePath, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(configListCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	packageFileName := strings.TrimSpace(output)

	return packageFileName, nil
}

func createPackCmdConfig(executablePath string, splitFlags []string) *npmutils.NpmConfig {
	return &npmutils.NpmConfig{
		Npm:          executablePath,
		Command:      []string{"pack"},
		CommandFlags: append(splitFlags),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
