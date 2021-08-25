package oc

import (
	"encoding/json"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func Version(executablePath string) (string, error) {
	versionCmdConfig := createVersionCmdConfig(executablePath)
	output, err := gofrogcmd.RunCmdOutput(versionCmdConfig)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	var versionRes ocVersionResponse
	err = json.Unmarshal([]byte(output), &versionRes)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return versionRes.ClientVersion.GitVersion, nil
}

func createVersionCmdConfig(executablePath string) *OcCommandConfig {
	return &OcCommandConfig{
		Executable:   executablePath,
		Command:      []string{"version"},
		CommandFlags: []string{"-o=json"},
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}

type ocVersionResponse struct {
	ClientVersion clientVersion `json:"clientVersion,omitempty"`
}

type clientVersion struct {
	GitVersion string `json:"gitVersion,omitempty"`
}
