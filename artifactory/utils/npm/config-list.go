package npm

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"io"
)

// This method runs "npm c ls" command and returns the current npm configuration (calculated by all flags and .npmrc files).
// For more info see https://docs.npmjs.com/cli/config
func GetConfigList(npmFlags []string, executablePath string) (data []byte, err error) {
	pipeReader, pipeWriter := io.Pipe()
	defer func(pipeReader *io.PipeReader) {
		e := pipeReader.Close()
		if err == nil {
			err = e
		}
	}(pipeReader)

	npmFlags = append(npmFlags, "--json=false")
	configListCmdConfig := createConfigListCmdConfig(executablePath, npmFlags, pipeWriter)
	npmErrorChan := make(chan error, 1)
	go func() {
		npmErrorChan <- gofrogcmd.RunCmd(configListCmdConfig)
	}()

	data, err = io.ReadAll(pipeReader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	npmError := <-npmErrorChan
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
