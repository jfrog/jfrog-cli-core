package yarn

import (
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const unsupportedYarnVersion = "4.0.0"

func IsInstalledYarnVersionSupported(executablePath string) error {
	versionGetCmdConfig := getVersionCmdConfig(executablePath)
	output, err := gofrogcmd.RunCmdOutput(versionGetCmdConfig)
	if err != nil {
		return errorutils.CheckError(err)
	}
	yarnVersion := strings.TrimSpace(output)
	return IsVersionSupported(yarnVersion)
}

func IsVersionSupported(versionStr string) error {
	yarnVersion := version.NewVersion(versionStr)
	if yarnVersion.Compare(unsupportedYarnVersion) <= 0 {
		return errorutils.CheckErrorf("Yarn version 4 is not supported. The current version is: %s. Please downgrade to a compatible version to continue", versionStr)
	}
	return nil
}

func getVersionCmdConfig(executablePath string) *YarnConfig {
	return &YarnConfig{
		Executable:   executablePath,
		Command:      []string{"--version"},
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}
