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

// Extracts packed file names from npm pack command output
// The output can differ when a prePack script exists,
// This function will filter the output and search for the .tgz files.
func getPackageFileNameFromOutput(output string) []string {
	// Split the output into lines
	lines := strings.Split(output, "\n")
	// Filter the lines to only include those that end with .tgz
	var packagesFileNames []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// If the line ends with .tgz, add it to the tgzFiles slice
		if strings.HasSuffix(line, ".tgz") {
			packagesFileNames = append(packagesFileNames, line)
		}
	}
	return packagesFileNames
}
