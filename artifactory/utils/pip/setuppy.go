package pip

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
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
		return "", errorutils.CheckError(errors.New("Failed extracting package name from content."))
	}

	return match[1], nil
}

// Run egg-info command on setup.py, the command generates metadata files.
// Return the content of the 'PKG-INFO' file.
func getEgginfoPkginfoContent(setuppyFilePath, pythonExecutablePath string) ([]byte, error) {
	eggBase, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		e := fileutils.RemoveTempDir(eggBase)
		if err == nil {
			err = e
		}
	}()

	// Run python 'egg_info --egg-base <eggBase>' command.
	if err = executeEgginfo(pythonExecutablePath, setuppyFilePath, eggBase); err != nil {
		return nil, errorutils.CheckError(err)
	}

	// Read PKG_INFO under <eggBase>/*.egg-info/PKG-INFO.
	return extractPackageNameFromEggBase(eggBase)
}

// Parse the output of 'python egg_info' command, in order to find the path of generated file 'PKG-INFO'.
func extractPackageNameFromEggBase(eggBase string) ([]byte, error) {
	files, err := ioutil.ReadDir(eggBase)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".egg-info") {
			pkginfoPath := filepath.Join(eggBase, file.Name(), "PKG-INFO")
			// Read PKG-INFO file.
			pkginfoFileExists, err := fileutils.IsFileExists(pkginfoPath, false)
			if errorutils.CheckError(err) != nil {
				return nil, err
			}
			if !pkginfoFileExists {
				return nil, errorutils.CheckError(errors.New("File 'PKG-INFO' couldn't be found in its designated location: " + pkginfoPath))
			}

			return ioutil.ReadFile(pkginfoPath)
		}
	}

	return nil, errorutils.CheckError(errors.New("couldn't find pkg info files"))
}

// Execute egg_info command for setup.py.
func executeEgginfo(pythonExecutablePath, setuppyFilePath, tempDirPath string) error {
	return exec.Command(pythonExecutablePath, setuppyFilePath, "egg_info", "--egg-base", tempDirPath).Run()
}
