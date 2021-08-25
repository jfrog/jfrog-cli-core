package oc

import (
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func StartBuild(executablePath string, ocFlags []string) (ocBuildName string, err error) {
	startBuildCmdConfig := createStartBuildCmdConfig(executablePath, ocFlags)
	output, err := gofrogcmd.RunCmdOutput(startBuildCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	ocBuildName = strings.TrimSpace(output)
	return
}

func createStartBuildCmdConfig(executablePath string, splitFlags []string) *OcCommandConfig {
	return &OcCommandConfig{
		Executable:   executablePath,
		Command:      []string{"start-build"},
		CommandFlags: append(splitFlags, "-w", "--template={{.metadata.name}}"),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
