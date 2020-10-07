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

func (gc *GoCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	if gc.deployerParams != nil && !gc.deployerParams.IsRtDetailsEmpty() {
		return gc.deployerParams.RtDetails()
	}
	return gc.resolverParams.RtDetails()
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

	resolverDetails, err := gc.resolverParams.RtDetails()
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
		deployerDetails, err := gc.deployerParams.RtDetails()
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
				// Package name was not supply, invalid go get commend
				return errorutils.CheckError(errors.New("invalid get command, package name is missing"))
			}
			rtDetails, err := resolverDetails.CreateArtAuthConfig()
			tempDirPath, err = fileutils.CreateTempDir()
			if err != nil {
				return err
			}
			err = copyGoPackageFiles(tempDirPath, gc.goArg[1], gc.resolverParams.TargetRepo(), rtDetails)
			if err != nil {
				return err
			}
			// Cleanup the temp working directory
			defer fileutils.RemoveTempDir(tempDirPath)
		}
		// The version is not necessary because we are collecting the dependencies only.
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
func copyGoPackageFiles(destPath, packageName, rtTargetRepo string, rtDetails auth.ServiceDetails) error {
	var packageFilesPath, version string
	packageCachePath, err := executersutils.GetPackagePath()
	if err != nil {
		return err
	}
	packageNameSplitted := strings.Split(packageName, "@")
	// The Case the user ask for specifc version
	if len(packageNameSplitted) == 2 && strings.HasSuffix(packageNameSplitted[1], "v") {
		version = packageNameSplitted[1]
	} else {
		// find the version using Artifactory
		packageName = filepath.Join(packageNameSplitted[0], "@v")
		if len(packageNameSplitted) == 1 {
			// no version was given to get command, so the latest was downloaded
			packageName = path.Join(packageName, "latest.info")
		} else {
			// A branch name was given by the user
			packageName = path.Join(packageName, packageNameSplitted[1]+".info")
		}

		version, err = executersutils.GetPackageVersion(rtTargetRepo, packageName, rtDetails)
		if err != nil {
			return err
		}
	}
	packageFilesPath = filepath.Join(packageCachePath, packageNameSplitted[0]+"@"+version)
	// Copy the entire content of the relevant Go pkg directory to the requested destination path.
	err = fileutils.CopyDir(packageFilesPath, destPath, true, nil)
	if err != nil {
		return fmt.Errorf("Couldn't find suitable package files: %s", packageFilesPath)
	}
	return nil

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
