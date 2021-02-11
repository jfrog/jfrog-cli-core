package golang

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/jfrog/gocmd"
	executersutils "github.com/jfrog/gocmd/executers/utils"
	"github.com/jfrog/gocmd/params"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang/project"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	_go "github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const GoCommandName = "rt_go"

type GoCommand struct {
	noRegistry         bool
	publishDeps        bool
	goArg              []string
	buildConfiguration *utils.BuildConfiguration
	deployerParams     *utils.RepositoryConfig
	resolverParams     *utils.RepositoryConfig
}

func NewGoCommand() *GoCommand {
	return &GoCommand{}
}

func (gc *GoCommand) SetResolverParams(resolverParams *utils.RepositoryConfig) *GoCommand {
	gc.resolverParams = resolverParams
	return gc
}

func (gc *GoCommand) SetDeployerParams(deployerParams *utils.RepositoryConfig) *GoCommand {
	gc.deployerParams = deployerParams
	return gc
}

func (gc *GoCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *GoCommand {
	gc.buildConfiguration = buildConfiguration
	return gc
}

func (gc *GoCommand) SetNoRegistry(noRegistry bool) *GoCommand {
	gc.noRegistry = noRegistry
	return gc
}

func (gc *GoCommand) SetPublishDeps(publishDeps bool) *GoCommand {
	gc.publishDeps = publishDeps
	return gc
}

func (gc *GoCommand) SetGoArg(goArg []string) *GoCommand {
	gc.goArg = goArg
	return gc
}

func (gc *GoCommand) ServerDetails() (*config.ServerDetails, error) {
	if gc.deployerParams != nil && !gc.deployerParams.IsServerDetailsEmpty() {
		return gc.deployerParams.ServerDetails()
	}
	return gc.resolverParams.ServerDetails()
}

func (gc *GoCommand) CommandName() string {
	return GoCommandName
}

func (gc *GoCommand) Run() error {
	err := golang.LogGoVersion()
	if err != nil {
		return err
	}
	buildName := gc.buildConfiguration.BuildName
	buildNumber := gc.buildConfiguration.BuildNumber
	isCollectBuildInfo := len(buildName) > 0 && len(buildNumber) > 0
	if isCollectBuildInfo {
		err = utils.SaveBuildGeneralDetails(buildName, buildNumber)
		if err != nil {
			return err
		}
	}

	resolverDetails, err := gc.resolverParams.ServerDetails()
	if err != nil {
		return err
	}
	resolverServiceManager, err := utils.CreateServiceManager(resolverDetails, false)
	if err != nil {
		return err
	}
	resolverParams := &params.Params{}
	resolverParams.SetRepo(gc.resolverParams.TargetRepo()).SetServiceManager(resolverServiceManager)
	goInfo := &params.ResolverDeployer{}
	goInfo.SetResolver(resolverParams)
	var targetRepo string
	var deployerServiceManager artifactory.ArtifactoryServicesManager
	if gc.publishDeps {
		deployerDetails, err := gc.deployerParams.ServerDetails()
		if err != nil {
			return err
		}
		deployerServiceManager, err = utils.CreateServiceManager(deployerDetails, false)
		if err != nil {
			return err
		}
		targetRepo = gc.deployerParams.TargetRepo()
		deployerParams := &params.Params{}
		deployerParams.SetRepo(targetRepo).SetServiceManager(deployerServiceManager)
		goInfo.SetDeployer(deployerParams)
	}

	err = gocmd.RunWithFallbacksAndPublish(gc.goArg, gc.noRegistry, gc.publishDeps, goInfo)
	if err != nil {
		return err
	}
	if isCollectBuildInfo {
		tempDirPath := ""
		if isGoGetCommand := len(gc.goArg) > 0 && gc.goArg[0] == "get"; isGoGetCommand {
			if len(gc.goArg) < 2 {
				// Package name was not supplied. Invalid go get commend
				return errorutils.CheckError(errors.New("Invalid get command. Package name is missing"))
			}
			tempDirPath, err = fileutils.CreateTempDir()
			if err != nil {
				return err
			}
			// Cleanup the temp working directory at the end.
			defer fileutils.RemoveTempDir(tempDirPath)
			// Get Artifactory config in order to extract the package version.
			serverDetails, err := resolverDetails.CreateArtAuthConfig()
			if err != nil {
				return err
			}
			err = copyGoPackageFiles(tempDirPath, gc.goArg[1], gc.resolverParams.TargetRepo(), serverDetails)
			if err != nil {
				return err
			}
		}
		goProject, err := project.Load("-", tempDirPath)
		if err != nil {
			return err
		}
		includeInfoFiles, err := shouldIncludeInfoFiles(deployerServiceManager, resolverServiceManager)
		if err != nil {
			return err
		}
		err = goProject.LoadDependencies()
		if err != nil {
			return err
		}
		err = goProject.CreateBuildInfoDependencies(includeInfoFiles)
		if err != nil {
			return err
		}
		err = utils.SaveBuildInfo(buildName, buildNumber, goProject.BuildInfo(false, gc.buildConfiguration.Module, targetRepo))
	}

	return err
}

// copyGoPackageFiles copies the package files from the go mod cache directory to the given destPath.
// The path to those chache files is retrived using the supplied package name and Artifactory details.
func copyGoPackageFiles(destPath, packageName, rtTargetRepo string, authArtDetails auth.ServiceDetails) error {
	packageFilesPath, err := getPackageFilePathFromArtifactory(packageName, rtTargetRepo, authArtDetails)
	if err != nil {
		return err
	}
	// Copy the entire content of the relevant Go pkg directory to the requested destination path.
	err = fileutils.CopyDir(packageFilesPath, destPath, true, nil)
	if err != nil {
		return fmt.Errorf("Couldn't find suitable package files: %s", packageFilesPath)
	}
	return nil
}

// getPackageFilePathFromArtifactory returns a string that represents the package files chache path.
// In most cases the path to those chache files is retrived using the supplied package name and Artifactory details.
// However if the user asked for a specifc version (package@vX.Y.Z) the unnecessary call to Artifactpry is avoided.
func getPackageFilePathFromArtifactory(packageName, rtTargetRepo string, authArtDetails auth.ServiceDetails) (packageFilesPath string, err error) {
	var version string
	packageCachePath, err := executersutils.GetGoModCachePath()
	if err != nil {
		return
	}
	packageNameSplitted := strings.Split(packageName, "@")
	name := packageNameSplitted[0]
	// The case the user asks for a specifc version
	if len(packageNameSplitted) == 2 && strings.HasPrefix(packageNameSplitted[1], "v") {
		version = packageNameSplitted[1]
	} else {
		branchName := ""
		// The case the user asks for a specifc branch
		if len(packageNameSplitted) == 2 {
			branchName = packageNameSplitted[1]
		}
		packageVersionRequest := buildPackageVersionRequest(name, branchName)
		// Retrive the package version using Artifactory
		version, err = executersutils.GetPackageVersion(rtTargetRepo, packageVersionRequest, authArtDetails)
		if err != nil {
			return
		}
	}
	return filepath.Join(packageCachePath, name+"@"+version), nil

}

// buildPackageVersionRequest returns a string representing the version request to Artifactory.
// The resulted string is in the following format: "<Package Name>/@V/<Branch Name>.info".
// If a branch name is not given, the branch name will be replaced with the "latest" keyword.
// ("<Package Name>/@V/latest.info").
func buildPackageVersionRequest(name, branchName string) string {
	packageVersionRequest := path.Join(name, "@v")
	if branchName != "" {
		// A branch name was given by the user
		return path.Join(packageVersionRequest, branchName+".info")
	}
	// No version was given to "go get" command, so the latest version should be requested
	return path.Join(packageVersionRequest, "latest.info")
}

// Returns true/false if info files should be included in the build info.
func shouldIncludeInfoFiles(deployerServiceManager artifactory.ArtifactoryServicesManager, resolverServiceManager artifactory.ArtifactoryServicesManager) (bool, error) {
	var artifactoryVersion string
	var err error
	if deployerServiceManager != nil {
		artifactoryVersion, err = deployerServiceManager.GetConfig().GetServiceDetails().GetVersion()
	} else {
		artifactoryVersion, err = resolverServiceManager.GetConfig().GetServiceDetails().GetVersion()
	}
	if err != nil {
		return false, err
	}
	version := version.NewVersion(artifactoryVersion)
	includeInfoFiles := version.AtLeast(_go.ArtifactoryMinSupportedVersionForInfoFile)
	return includeInfoFiles, nil
}
