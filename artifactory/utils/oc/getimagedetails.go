package oc

import (
	"fmt"
	"github.com/pkg/errors"
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func GetImageDetails(executablePath, ocBuildName string) (imageTag, manifestSha256 string, err error) {
	getImageDetailsCmdConfig := createGetImageDetailsCmdConfig(executablePath, ocBuildName)
	output, err := gofrogcmd.RunCmdOutput(getImageDetailsCmdConfig)
	if err != nil {
		return "", "", errorutils.CheckError(err)
	}
	splitOutput := strings.Split(strings.TrimSpace(output), "@")
	if len(splitOutput) != 2 {
		return "", "", errorutils.CheckError(errors.New(fmt.Sprintf("Unable to parse image tag and digest of build %s. Output from OpenShift CLI: %s", ocBuildName, output)))
	}

	return splitOutput[0], splitOutput[1], nil
}

func createGetImageDetailsCmdConfig(executablePath, ocBuildName string) *OcCommandConfig {
	return &OcCommandConfig{
		Executable:   executablePath,
		Command:      []string{"get", "build", ocBuildName},
		CommandFlags: []string{"--template={{.status.outputDockerImageReference}}@{{.status.output.to.imageDigest}}"},
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
