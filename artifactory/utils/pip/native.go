package pip

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os"
)

// NativeExecutor handles the execution of any pip command which is not "install".
type NativeExecutor struct {
	CmdName string
	CommonExecutor
}

func (pne *NativeExecutor) Run() error {
	// Prepare for running.
	pipExecutablePath, pipIndexUrl, err := pne.prepare()
	if err != nil {
		return err
	}

	// Run pip.
	err = pne.runPipNative(pipExecutablePath, pipIndexUrl)
	if err != nil {
		return err
	}

	return nil
}

func (pne *NativeExecutor) runPipNative(pipExecutablePath, pipIndexUrl string) error {
	pipCmd := &PipCmd{
		Executable:  pipExecutablePath,
		Command:     pne.CmdName,
		CommandArgs: append(pne.Args, "-i", pipIndexUrl),
	}

	command := pipCmd.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
