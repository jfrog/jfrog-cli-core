package oc

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os/exec"
)

func Version(executablePath string) (string, error) {
	cmdArgs := []string{"version", "-o=json"}
	outputBytes, err := exec.Command(executablePath, cmdArgs...).Output()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	var versionRes ocVersionResponse
	err = json.Unmarshal(outputBytes, &versionRes)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return versionRes.ClientVersion.GitVersion, nil
}

type ocVersionResponse struct {
	ClientVersion clientVersion `json:"clientVersion,omitempty"`
}

type clientVersion struct {
	GitVersion string `json:"gitVersion,omitempty"`
}
