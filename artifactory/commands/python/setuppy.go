package python

import (
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
)

// Get the project-name by running 'egg_info' command on setup.py and extracting it from 'PKG-INFO' file.
func ExtractPackageNameFromSetupPy(setuppyFilePath, pythonExecutablePath string) (string, error) {
	// Execute egg_info command and return PKG-INFO content.
	content, err := getEgginfoPkginfoContent(setuppyFilePath, pythonExecutablePath)
	if err != nil {
		return "", err
	}

	// Extract project name from file content.
	return getProjectNameFromFileContent(content)
}

// Get package-name from PKG-INFO file content.
// If pattern of package-name not found, return an error.
func getProjectNameFromFileContent(content []byte) (string, error) {
	// Create package-name regexp.
	packageNameRegexp, err := utils.GetRegExp(`(?m)^Name\:\s(\w[\w-\.]+)`)
	if err != nil {
		return "", err
	}

	// Find first match of packageNameRegexp.
	match := packageNameRegexp.FindStringSubmatch(string(content))
	if len(match) < 2 {
		return "", errorutils.CheckErrorf("Failed extracting package name from content.")
	}

	return match[1], nil
}

// Run egg-info command on setup.py, the command generates metadata files.
// Return the content of the 'PKG-INFO' file.
func getEgginfoPkginfoContent(setuppyFilePath, pythonExecutablePath string) (output []byte, err error) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		e := fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()

	// Run python egg_info command.
	egginfoOutput, err := executeEgginfoCommandWithOutput(pythonExecutablePath, setuppyFilePath, tempDirPath)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	// Parse egg_info execution output to find PKG-INFO path.
	pkginfoPath, err := extractPkginfoPathFromCommandOutput(egginfoOutput)
	if err != nil {
		return nil, err
	}

	// Read PKG-INFO file.
	pkginfoFileExists, err := fileutils.IsFileExists(pkginfoPath, false)
	if !pkginfoFileExists {
		return nil, errorutils.CheckErrorf("File 'PKG-INFO' couldn't be found in its designated location: %s", pkginfoPath)
	}

	output, err = ioutil.ReadFile(pkginfoPath)
	return
}

// Parse the output of 'python egg_info' command, in order to find the path of generated file 'PKG-INFO'.
func extractPkginfoPathFromCommandOutput(egginfoOutput string) (string, error) {
	// Regexp for extracting 'PKG-INFO' file-path from the 'egg_info' command output.
	pkginfoRegexp, err := utils.GetRegExp(`(?m)writing\s(\S+\.egg\-info[\\\/]PKG-INFO)`)
	if err != nil {
		return "", err
	}

	matchedOutputLines := pkginfoRegexp.FindAllString(egginfoOutput, -1)
	if len(matchedOutputLines) != 1 {
		return "", errorutils.CheckErrorf("Failed parsing egg_info command, couldn't find PKG-INFO location.")
	}

	// Extract path from matched line.
	matchedResults := pkginfoRegexp.FindStringSubmatch(matchedOutputLines[0])
	return matchedResults[1], nil
}

// Execute egg_info command for setup.py, return command's output.
func executeEgginfoCommandWithOutput(pythonExecutablePath, setuppyFilePath, tempDirPath string) (string, error) {
	pythonEggInfoCmd := &PythonCommand{
		executable:  pythonExecutablePath,
		commandName: setuppyFilePath,
		args:        []string{"egg_info", "--egg-base", tempDirPath},
	}
	return gofrogcmd.RunCmdOutput(pythonEggInfoCmd)
}
