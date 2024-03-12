package npm

import (
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func Pack(npmFlags []string, executablePath string) ([]string, error) {
	configListCmdConfig := createPackCmdConfig(executablePath, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(configListCmdConfig)
	if err != nil {
		return []string{}, errorutils.CheckError(err)
	}
	return getPackageFileNameFromOutput(output), nil
}

func createPackCmdConfig(executablePath string, splitFlags []string) *npmutils.NpmConfig {
	return &npmutils.NpmConfig{
		Npm:          executablePath,
		Command:      []string{"pack"},
		CommandFlags: append(splitFlags, "--json=false"),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}

func getPackageFileNameFromOutput(output string) []string {
	output = strings.TrimSpace(output)
	return strings.Split(output, "\n")
}
