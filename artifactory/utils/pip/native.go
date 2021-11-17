package pip

import gofrogcmd "github.com/jfrog/gofrog/io"

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

	// Run without log parsing.
	return gofrogcmd.RunCmd(pipCmd)
}
