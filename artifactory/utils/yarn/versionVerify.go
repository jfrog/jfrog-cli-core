package yarn

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

const UNSUPPORTED_YARN_VERSION = "4.0.0"

func VerifyYarnVersionSupport(executablePath string) error {
	versionGetCmdConfig := getVersionCmdConfig(executablePath)
	output, err := gofrogcmd.RunCmdOutput(versionGetCmdConfig)
	if err != nil {
		return errorutils.CheckError(err)
	}
	yarnVersionStr := strings.TrimSpace(output)
	return IsVersionSupported(yarnVersionStr)
}

func IsVersionSupported(versionStr string) error {
	yarnVersion := version.NewVersion(versionStr)
	if yarnVersion.Compare(UNSUPPORTED_YARN_VERSION) < 0 {
		return errorutils.CheckErrorf("Yarn version 4 is not supported. The current version is: " + versionStr +
			". Please downgrade to a compatible version to continue")
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
