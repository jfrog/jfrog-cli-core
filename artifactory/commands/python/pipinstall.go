package python

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"strings"
)

type PipInstallCommand struct {
	*PythonCommand
}

func NewPipInstallCommand() *PipInstallCommand {
	return &PipInstallCommand{PythonCommand: &PythonCommand{}}
}

func (pic *PipInstallCommand) Run() (err error) {
	log.Info("Running pip Install...")

	if err = pic.prepareBuildPrerequisites(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			pic.cleanBuildInfoDir()
		}
	}()

	pipExecutablePath, err := getExecutablePath("pip")
	if err != nil {
		return err
	}

	err = pic.setPypiRepoUrlWithCredentials(pic.serverDetails, pic.repository, utils.Pip)
	if err != nil {
		return err
	}

	pic.executable = pipExecutablePath
	pic.commandName = "install"

	if !pic.shouldCollectBuildInfo {
		err = gofrogcmd.RunCmd(pic)
		if err != nil {
			return err
		}
	} else {
		dependencyToFileMap, err := pic.runInstallWithLogParsing()
		if err != nil {
			return err
		}
		// Collect build-info.
		projectsDirPath, err := os.Getwd()
		if err != nil {
			return err
		}
		allDependencies := pic.getAllDependencies(dependencyToFileMap)
		if err := pic.collectBuildInfo(projectsDirPath, allDependencies); err != nil {
			return err
		}
	}
	log.Info("pip install finished successfully.")
	return nil
}

// Run pip-install command while parsing the logs for downloaded packages.
// Supports running pip either in non-verbose and verbose mode.
// Populates 'dependencyToFileMap' with downloaded package-name and its actual downloaded file (wheel/egg/zip...).
func (pic *PipInstallCommand) runInstallWithLogParsing() (map[string]string, error) {
	// Create regular expressions for log parsing.
	collectingPackageRegexp, err := clientutils.GetRegExp(`^Collecting\s(\w[\w-\.]+)`)
	if err != nil {
		return nil, err
	}
	downloadFileRegexp, err := clientutils.GetRegExp(`^\s\sDownloading\s[^\s]*\/([^\s]*)`)
	if err != nil {
		return nil, err
	}
	installedPackagesRegexp, err := clientutils.GetRegExp(`^Requirement\salready\ssatisfied\:\s(\w[\w-\.]+)`)
	if err != nil {
		return nil, err
	}

	downloadedDependencies := make(map[string]string)
	var packageName string
	expectingPackageFilePath := false

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

			// Save dependency information.
			filePath := pattern.MatchedResults[1]
			downloadedDependencies[strings.ToLower(packageName)] = filePath
			expectingPackageFilePath = false

			log.Debug(fmt.Sprintf("Found package: %s installed with: %s", packageName, filePath))
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

			// Save dependency with empty file name.
			downloadedDependencies[strings.ToLower(pattern.MatchedResults[1])] = ""

			log.Debug(fmt.Sprintf("Found package: %s already installed", pattern.MatchedResults[1]))
			return pattern.Line, nil
		},
	}

	// Execute command.
	_, _, _, err = gofrogcmd.RunCmdWithOutputParser(pic, true, &dependencyNameParser, &dependencyFileParser, &installedPackagesParser)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}

	return downloadedDependencies, nil
}

// Convert dependencyToFileMap to Dependencies map.
func (pic *PipInstallCommand) getAllDependencies(dependencyToFileMap map[string]string) map[string]*buildinfo.Dependency {
	dependenciesMap := make(map[string]*buildinfo.Dependency, len(dependencyToFileMap))
	for depName, fileName := range dependencyToFileMap {
		dependenciesMap[depName] = &buildinfo.Dependency{Id: fileName}
	}

	return dependenciesMap
}

func (pic *PipInstallCommand) CommandName() string {
	return "rt_pip_install"
}

func (pic *PipInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return pic.serverDetails, nil
}
