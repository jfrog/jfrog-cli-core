package npm

import (
	"io"
	"io/ioutil"

	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// This method runs "npm c ls" command and returns the current npm configuration (calculated by all flags and .npmrc files).
// For more info see https://docs.npmjs.com/cli/config
func GetConfigList(npmFlags []string, executablePath string) ([]byte, error) {
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	npmFlags = append(npmFlags, "--json=false")
	configListCmdConfig := createConfigListCmdConfig(executablePath, npmFlags, pipeWriter)
	var npmError error
	go func() {
		npmError = gofrogcmd.RunCmd(configListCmdConfig)
	}()

	data, err := ioutil.ReadAll(pipeReader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	if npmError != nil {
		return nil, errorutils.CheckError(npmError)
	}
	return data, nil
}

func createConfigListCmdConfig(executablePath string, splitFlags []string, pipeWriter *io.PipeWriter) *npmutils.NpmConfig {
	return &npmutils.NpmConfig{
		Npm:          executablePath,
		Command:      []string{"c", "ls"},
		CommandFlags: splitFlags,
		StrWriter:    pipeWriter,
		ErrWriter:    nil,
	}
}
