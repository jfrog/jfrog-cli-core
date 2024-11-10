package npm

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	npmPackedTarballSuffix = ".tgz"
)

func Pack(npmFlags []string, executablePath string) ([]string, error) {
	configListCmdConfig := createPackCmdConfig(executablePath, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(configListCmdConfig)
	if err != nil {
		return []string{}, errorutils.CheckError(err)
	}
	return getPackageFileNameFromOutput(output)
}

func createPackCmdConfig(executablePath string, splitFlags []string) *NpmConfig {
	return &NpmConfig{
		Npm:          executablePath,
		Command:      []string{"pack"},
		CommandFlags: append(splitFlags, "--json=false"),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}

// Extracts packed file names from npm pack command output
// The output can differ when a prePack script exists,
// This function will filter the output and search for the .tgz files
// To avoid misidentifying files, we will verify file exists
func getPackageFileNameFromOutput(output string) (packedTgzFiles []string, err error) {
	lines := strings.Split(output, "\n")
	var packedFileNamesFromOutput []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, npmPackedTarballSuffix) {
			packedFileNamesFromOutput = append(packedFileNamesFromOutput, line)
		}
	}
	for _, file := range packedFileNamesFromOutput {
		exists, err := fileutils.IsFileExists(file, true)
		if err != nil {
			return nil, fmt.Errorf("error occurred while checking packed npm tarball exists: %w", err)
		}
		if exists {
			packedTgzFiles = append(packedTgzFiles, file)
		}
	}
	return
}
