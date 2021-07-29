package npm

import (
	"errors"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"strings"
)

const npmPackJsonFlagSupportMinVersion = "7.14.0"

func Pack(npmFlags []string, executablePath string, npmVersion *version.Version) (string, error) {
	// On supported npm versions we extract file name using regexp on a json output.
	jsonFlagSupported := isJsonFlagSupported(npmVersion)
	if jsonFlagSupported {
		npmFlags = append(npmFlags, "--json")
	}
	packCmd := createPackCmdConfig(executablePath, npmFlags)
	output, err := gofrogcmd.RunCmdOutput(packCmd)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return getPackageFileNameFromOutput(output, jsonFlagSupported)
}

func createPackCmdConfig(executablePath string, splitFlags []string) *NpmConfig {
	return &NpmConfig{
		Npm:          executablePath,
		Command:      []string{"pack"},
		CommandFlags: append(splitFlags),
		StrWriter:    nil,
		ErrWriter:    nil,
	}
}

func isJsonFlagSupported(npmVersion *version.Version) bool {
	if npmVersion == nil || npmVersion.Compare(npmPackJsonFlagSupportMinVersion) > 0 {
		return false
	}
	return true
}

func getPackageFileNameFromOutput(output string, isJsonFlagSupported bool) (string, error) {
	output = strings.TrimSpace(output)
	if !isJsonFlagSupported {
		lines := strings.Split(output, "\n")
		return strings.TrimSpace(lines[len(lines)-1]), nil
	}

	filenameRegexp, err := utils.GetRegExp(`"filename": "(.*).tgz",`)
	if err != nil {
		return "", err
	}

	match := filenameRegexp.FindStringSubmatch(output)
	if len(match) < 2 {
		return "", errorutils.CheckError(errors.New("failed extracting filename from pack output"))
	}

	return match[1] + ".tgz", nil
}
