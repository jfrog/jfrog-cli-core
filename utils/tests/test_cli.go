package tests

import (
	"io"
	"os"
	"strings"
	"testing"

	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

type JfrogCli struct {
	main        func() error
	prefix      string
	credentials string
}

func NewJfrogCli(mainFunc func() error, prefix, credentials string) *JfrogCli {
	return &JfrogCli{mainFunc, prefix, credentials}
}

func (cli *JfrogCli) SetPrefix(prefix string) *JfrogCli {
	cli.prefix = prefix
	return cli
}

func (cli *JfrogCli) Exec(args ...string) error {
	spaceSplit := " "
	os.Args = strings.Split(cli.prefix, spaceSplit)
	output := strings.Split(cli.prefix, spaceSplit)
	for _, v := range args {
		if v == "" {
			continue
		}
		args := strings.Split(v, spaceSplit)
		os.Args = append(os.Args, v)
		output = append(output, args...)
	}
	if cli.credentials != "" {
		args := strings.Split(cli.credentials, spaceSplit)
		os.Args = append(os.Args, args...)
	}

	log.Info("[Command]", strings.Join(output, " "))
	return cli.main()
}

// Run `jfrog` command, redirect the stdout and return the output
func (cli *JfrogCli) RunCliCmdWithOutput(t *testing.T, args ...string) string {
	return RunCmdWithOutput(t, func() error { return cli.Exec(args...) })
}

func (cli *JfrogCli) RunCliCmdWithOutputs(t *testing.T, args ...string) (string, error) {
	return RunCmdWithOutputs(t, func() error { return cli.Exec(args...) })
}

func RunCmdWithOutputs(t *testing.T, executeCmd func() error) (output string, err error) {
	newStdout, stdWriter, cleanUp := redirectOutToPipe(t)
	defer cleanUp()

	go func() {
		err = executeCmd()
		// Closing the temp stdout in order to be able to read it's content.
		assert.NoError(t, stdWriter.Close())
	}()

	content, e := io.ReadAll(newStdout)
	assert.NoError(t, e)
	output = string(content)
	log.Debug(output)
	return
}

// Run a command, redirect the stdout and return the output
func RunCmdWithOutput(t *testing.T, executeCmd func() error) string {
	output, err := RunCmdWithOutputs(t, executeCmd)
	assert.NoError(t, err)
	return output
}

func redirectOutToPipe(t *testing.T) (*os.File, *os.File, func()) {
	newStdout, stdWriter, previousStdout := RedirectStdOutToPipe()
	previousLog := log.Logger
	log.SetLogger(log.NewLogger(corelog.GetCliLogLevel(), nil))
	// Restore previous stdout when the function returns
	return newStdout, stdWriter, func() {
		os.Stdout = previousStdout
		log.SetLogger(previousLog)
		assert.NoError(t, newStdout.Close())
	}
}

func (cli *JfrogCli) LegacyBuildToolExec(args ...string) error {
	spaceSplit := " "
	os.Args = strings.Split(cli.prefix, spaceSplit)
	os.Args = append(os.Args, args...)

	log.Info("[Command]", os.Args)

	if cli.credentials != "" {
		args := strings.Split(cli.credentials, spaceSplit)
		os.Args = append(os.Args, args...)
	}
	return cli.main()
}

func (cli *JfrogCli) WithoutCredentials() *JfrogCli {
	return &JfrogCli{cli.main, cli.prefix, ""}
}

// Redirect stdout to new temp, os.pipe
// Caller is responsible to close the pipe and to set the old stdout back.
func RedirectStdOutToPipe() (reader *os.File, writer *os.File, previousStdout *os.File) {
	previousStdout = os.Stdout
	reader, writer, _ = os.Pipe()
	os.Stdout = writer
	return
}
