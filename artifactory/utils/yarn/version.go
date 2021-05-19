package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"io"
	"io/ioutil"
)

func Version(executablePath string) ([]byte, error) {
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()
	defer pipeWriter.Close()
	var yarnError error

	versionCmdConfig := createVersionCmdConfig(executablePath, pipeWriter)
	go func() {
		yarnError = gofrogcmd.RunCmd(versionCmdConfig)
	}()

	data, err := ioutil.ReadAll(pipeReader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	if yarnError != nil {
		return nil, errorutils.CheckError(yarnError)
	}

	return data, nil
}

func createVersionCmdConfig(executablePath string, pipeWriter *io.PipeWriter) *YarnConfig {
	return &YarnConfig{
		Executable: executablePath,
		Command:    []string{"--version"},
		StrWriter:  pipeWriter,
		ErrWriter:  nil,
	}
}
