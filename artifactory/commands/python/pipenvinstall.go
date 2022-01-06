package python

import (
	"errors"
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	pipenvutils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type PipenvInstallCommand struct {
	*PythonCommand
}

func NewPipenvInstallCommand() *PipenvInstallCommand {
	return &PipenvInstallCommand{PythonCommand: &PythonCommand{}}
}

func (peic *PipenvInstallCommand) Run() (err error) {
	log.Info("Running pipenv Install...")

	if err = peic.prepareBuildPrerequisites(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			e := peic.cleanBuildInfoDir()
			if e != nil {
				err = errors.New(err.Error() + "\n" + e.Error())
			}
		}
	}()

	pipenvExecutablePath, err := getExecutablePath("pipenv")
	if err != nil {
		return err
	}

	peic.executable = pipenvExecutablePath
	peic.commandName = "install"

	err = peic.setPypiRepoUrlWithCredentials(peic.serverDetails, peic.repository, utils.Pipenv)
	if err != nil {
		return err
	}

	if !peic.shouldCollectBuildInfo {
		err = gofrogcmd.RunCmd(peic)
		if err != nil {
			return err
		}
	} else {
		// Add verbosity to get the needed log lines to parse the binary files fot build info
		peic.args = append(peic.args, "-v")

		dependencyToFileMap, err := peic.runInstallWithLogParsing()
		if err != nil {
			return err
		}
		dependenciesGraph, topLevelPackagesList, err := pipenvutils.GetPipenvDependenciesList("")
		if err != nil {
			return err
		}

		allDependencies := peic.getAllDependencies(dependenciesGraph, dependencyToFileMap)
		pythonExecutablePath, err := getExecutablePath("python")
		if err != nil {
			return err
		}
		venvDirPath, err := pipenvutils.GetPipenvVenv()
		if err != nil {
			return err
		}
		// Collect build-info.
		if err = peic.collectBuildInfo(venvDirPath, pythonExecutablePath, allDependencies, dependenciesGraph, topLevelPackagesList); err != nil {
			return err
		}
	}

	log.Info("pipenv install finished successfully.")
	return nil
}

// Convert dependencyToFileMap to Dependencies map.
func (peic *PipenvInstallCommand) getAllDependencies(allDepsList map[string][]string, dependencyToFileMap map[string]string) map[string]*buildinfo.Dependency {
	dependenciesMap := make(map[string]*buildinfo.Dependency, len(dependencyToFileMap))
	for depId := range allDepsList {
		depName := depId[0:strings.Index(depId, ":")]
		dependenciesMap[depName] = &buildinfo.Dependency{Id: dependencyToFileMap[depName]}
	}
	return dependenciesMap
}

// Run pipenv install command while parsing the logs for downloaded packages.
// Supports running pipenv either in non-verbose and verbose mode.
// Populates 'dependencyToFileMap' with downloaded package-name and its actual downloaded file (wheel/egg/zip...).
func (peic *PipenvInstallCommand) runInstallWithLogParsing() (map[string]string, error) {
	// Create regular expressions for log parsing.
	collectingPackageRegexp, err := clientutils.GetRegExp(`^Collecting\s(\w[\w-\.]+)`)
	if err != nil {
		return nil, err
	}
	downloadFileRegexp, err := clientutils.GetRegExp(`^\s\sDownloading\s(\S*)\s\(`)
	if err != nil {
		return nil, err
	}
	installedPackagesRegexp, err := clientutils.GetRegExp(`^\s\sUsing\scached\s([\S]+)\s\(`)
	if err != nil {
		return nil, err
	}

	downloadedDependencies := make(map[string]string)
	expectingPackageFilePath := false
	packageName := ""
	// Extract downloaded package name.
	dependencyNameParser := gofrogcmd.CmdOutputPattern{
		RegExp: collectingPackageRegexp,
		ExecFunc: func(pattern *gofrogcmd.CmdOutputPattern) (string, error) {
			// If this pattern matched a second time before downloaded-file-name was found, prompt a message.
			if expectingPackageFilePath {
				// This may occur when a package-installation file is saved in pip-cache-dir, thus not being downloaded during the installation.
				// Re-running pip-install with 'no-cache-dir' fixes this issue.
				log.Debug(fmt.Sprintf("Could not resolve download path for package: %s, continuing...", packageName))

				// Save package with empty file path.
				downloadedDependencies[strings.ToLower(packageName)] = ""
			}

			// Check for out of bound results.
			if len(pattern.MatchedResults)-1 < 0 {
				log.Debug(fmt.Sprintf("Failed extracting package name from line: %s", pattern.Line))
				return pattern.Line, nil
			}

			// Save dependency information.
			expectingPackageFilePath = true
			packageName = pattern.MatchedResults[1]

			return pattern.Line, nil
		},
	}

	// Extract downloaded file, stored in Artifactory.
	dependencyFileParser := gofrogcmd.CmdOutputPattern{
		RegExp: downloadFileRegexp,
		ExecFunc: func(pattern *gofrogcmd.CmdOutputPattern) (string, error) {
			// Check for out of bound results.
			if len(pattern.MatchedResults)-1 < 0 {
				log.Debug(fmt.Sprintf("Failed extracting download path from line: %s", pattern.Line))
				return pattern.Line, nil
			}

			// If this pattern matched before package-name was found, do not collect this path.
			if !expectingPackageFilePath {
				log.Debug(fmt.Sprintf("Could not resolve package name for download path: %s , continuing...", packageName))
				return pattern.Line, nil
			}

			// Save dependency information
			filePath := pattern.MatchedResults[1]
			lastSlashIndex := strings.LastIndex(filePath, "/")
			var fileName string
			if lastSlashIndex == -1 {
				fileName = filePath
			} else {
				fileName = filePath[lastSlashIndex+1:]
			}
			downloadedDependencies[strings.ToLower(packageName)] = fileName
			expectingPackageFilePath = false

			log.Debug(fmt.Sprintf("Found package: %s installed with: %s", packageName, fileName))
			return pattern.Line, nil
		},
	}

	// Extract already installed packages names.
	installedPackagesParser := gofrogcmd.CmdOutputPattern{
		RegExp: installedPackagesRegexp,
		ExecFunc: func(pattern *gofrogcmd.CmdOutputPattern) (string, error) {
			// Check for out of bound results.
			if len(pattern.MatchedResults)-1 < 0 {
				log.Debug(fmt.Sprintf("Failed extracting package name from line: %s", pattern.Line))
				return pattern.Line, nil
			}

			filePath := pattern.MatchedResults[1]
			lastSlashIndex := strings.LastIndex(filePath, "/")
			var fileName string
			if lastSlashIndex == -1 {
				fileName = filePath
			} else {
				fileName = filePath[lastSlashIndex+1:]
			}
			// Save dependency with empty file name.
			downloadedDependencies[strings.ToLower(packageName)] = fileName
			expectingPackageFilePath = false
			log.Debug(fmt.Sprintf("Found package: %s already installed", fileName))
			return pattern.Line, nil
		},
	}

	// Execute command.
	_, _, _, err = gofrogcmd.RunCmdWithOutputParser(peic, true, &dependencyNameParser, &dependencyFileParser, &installedPackagesParser)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	return downloadedDependencies, nil
}

func (peic *PipenvInstallCommand) CommandName() string {
	return "rt_pipenv_install"
}

func (peic *PipenvInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return peic.serverDetails, nil
}
