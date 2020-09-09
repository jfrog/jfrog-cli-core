package npm

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type NpmPublishCommandArgs struct {
	NpmCommand
	executablePath   string
	workingDirectory string
	collectBuildInfo bool
	packedFilePath   string
	packageInfo      *npm.PackageInfo
	publishPath      string
	tarballProvided  bool
	artifactData     []specutils.FileInfo
}

type NpmPublishCommand struct {
	configFilePath string
	commandName    string
	*NpmPublishCommandArgs
}

func NewNpmPublishCommand() *NpmPublishCommand {
	return &NpmPublishCommand{NpmPublishCommandArgs: NewNpmPublishCommandArgs(), commandName: "rt_npm_publish"}
}

func NewNpmPublishCommandArgs() *NpmPublishCommandArgs {
	return &NpmPublishCommandArgs{}
}

func (npc *NpmPublishCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return npc.rtDetails, nil
}

func (npc *NpmPublishCommand) SetConfigFilePath(configFilePath string) *NpmPublishCommand {
	npc.configFilePath = configFilePath
	return npc
}

func (nic *NpmPublishCommand) SetArgs(args []string) *NpmPublishCommand {
	nic.NpmPublishCommandArgs.npmArgs = args
	return nic
}

func (npc *NpmPublishCommand) Run() error {
	if npc.configFilePath != "" {
		// Read config file.
		log.Debug("Preparing to read the config file", npc.configFilePath)
		vConfig, err := utils.ReadConfigFile(npc.configFilePath, utils.YAML)
		if err != nil {
			return err
		}
		deployerParams, err := utils.GetRepoConfigByPrefix(npc.configFilePath, utils.ProjectConfigDeployerPrefix, vConfig)
		if err != nil {
			return err
		}
		_, _, filteredNpmArgs, buildConfiguration, err := npm.ExtractNpmOptionsFromArgs(npc.NpmPublishCommandArgs.npmArgs)
		if err != nil {
			return err
		}
		rtDetails, err := deployerParams.RtDetails()
		if err != nil {
			return errorutils.CheckError(err)
		}
		npc.SetBuildConfiguration(buildConfiguration).SetRepo(deployerParams.TargetRepo()).SetNpmArgs(filteredNpmArgs).SetRtDetails(rtDetails)
	}
	return npc.run()
}

func (npc *NpmPublishCommand) run() error {
	log.Info("Running npm Publish")
	if err := npc.preparePrerequisites(); err != nil {
		return err
	}

	if !npc.tarballProvided {
		if err := npc.pack(); err != nil {
			return err
		}
	}

	if err := npc.deploy(); err != nil {
		if npc.tarballProvided {
			return err
		}
		// We should delete the tarball we created
		return deleteCreatedTarballAndError(npc.packedFilePath, err)
	}

	if !npc.tarballProvided {
		if err := deleteCreatedTarball(npc.packedFilePath); err != nil {
			return err
		}
	}

	if !npc.collectBuildInfo {
		log.Info("npm publish finished successfully.")
		return nil
	}

	if err := npc.saveArtifactData(); err != nil {
		return err
	}

	log.Info("npm publish finished successfully.")
	return nil
}

func (npc *NpmPublishCommand) CommandName() string {
	return npc.commandName
}

func (npc *NpmPublishCommand) preparePrerequisites() error {
	log.Debug("Preparing prerequisites.")
	npmExecPath, err := exec.LookPath("npm")
	if err != nil {
		return errorutils.CheckError(err)
	}

	if npmExecPath == "" {
		return errorutils.CheckError(errors.New("Could not find 'npm' executable"))
	}

	npc.executablePath = npmExecPath
	log.Debug("Using npm executable:", npc.executablePath)
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}

	currentDir, err = filepath.Abs(currentDir)
	if err != nil {
		return errorutils.CheckError(err)
	}

	npc.workingDirectory = currentDir
	log.Debug("Working directory set to:", npc.workingDirectory)
	npc.collectBuildInfo = len(npc.buildConfiguration.BuildName) > 0 && len(npc.buildConfiguration.BuildNumber) > 0
	if err = npc.setPublishPath(); err != nil {
		return err
	}

	artDetails, err := npc.rtDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}

	if err = utils.CheckIfRepoExists(npc.repo, artDetails); err != nil {
		return err
	}

	return npc.setPackageInfo()
}

func (npc *NpmPublishCommand) pack() error {
	log.Debug("Creating npm package.")
	if err := npm.Pack(npc.npmArgs, npc.executablePath); err != nil {
		return err
	}

	npc.packedFilePath = filepath.Join(npc.workingDirectory, npc.packageInfo.GetExpectedPackedFileName())
	log.Debug("Created npm package at", npc.packedFilePath)
	return nil
}

func (npc *NpmPublishCommand) deploy() (err error) {
	log.Debug("Deploying npm package.")
	if err = npc.readPackageInfoFromTarball(); err != nil {
		return err
	}

	target := fmt.Sprintf("%s/%s", npc.repo, npc.packageInfo.GetDeployPath())
	artifactsFileInfo, err := npc.doDeploy(target, npc.rtDetails)
	if err != nil {
		return err
	}

	npc.artifactData = artifactsFileInfo
	return nil
}

func (npc *NpmPublishCommand) doDeploy(target string, artDetails *config.ArtifactoryDetails) (artifactsFileInfo []specutils.FileInfo, err error) {
	servicesManager, err := utils.CreateServiceManager(artDetails, false)
	if err != nil {
		return nil, err
	}
	up := services.UploadParams{}
	up.ArtifactoryCommonParams = &specutils.ArtifactoryCommonParams{Pattern: npc.packedFilePath, Target: target}
	if npc.collectBuildInfo {
		utils.SaveBuildGeneralDetails(npc.buildConfiguration.BuildName, npc.buildConfiguration.BuildNumber)
		props, err := utils.CreateBuildProperties(npc.buildConfiguration.BuildName, npc.buildConfiguration.BuildNumber)
		if err != nil {
			return nil, err
		}
		up.ArtifactoryCommonParams.Props = props
	}
	artifactsFileInfo, _, failed, err := servicesManager.UploadFiles(up)
	if err != nil {
		return nil, err
	}

	// We deploying only one Artifact which have to be deployed, in case of failure we should fail
	if failed > 0 {
		return nil, errorutils.CheckError(errors.New("Failed to upload the npm package to Artifactory. See Artifactory logs for more details."))
	}
	return artifactsFileInfo, nil
}

func (npc *NpmPublishCommand) saveArtifactData() error {
	log.Debug("Saving npm package artifact build info data.")
	var buildArtifacts []buildinfo.Artifact
	for _, artifact := range npc.artifactData {
		buildArtifacts = append(buildArtifacts, artifact.ToBuildArtifacts())
	}

	populateFunc := func(partial *buildinfo.Partial) {
		partial.Artifacts = buildArtifacts
		if npc.buildConfiguration.Module == "" {
			npc.buildConfiguration.Module = npc.packageInfo.BuildInfoModuleId()
		}
		partial.ModuleId = npc.buildConfiguration.Module
	}
	return utils.SavePartialBuildInfo(npc.buildConfiguration.BuildName, npc.buildConfiguration.BuildNumber, populateFunc)
}

func (npc *NpmPublishCommand) setPublishPath() error {
	log.Debug("Reading Package Json.")

	npc.publishPath = npc.workingDirectory
	if len(npc.npmArgs) > 0 && !strings.HasPrefix(strings.TrimSpace(npc.npmArgs[0]), "-") {
		path := strings.TrimSpace(npc.npmArgs[0])
		path = clientutils.ReplaceTildeWithUserHome(path)

		if filepath.IsAbs(path) {
			npc.publishPath = path
		} else {
			npc.publishPath = filepath.Join(npc.workingDirectory, path)
		}
	}
	return nil
}

func (npc *NpmPublishCommand) setPackageInfo() error {
	log.Debug("Setting Package Info.")
	fileInfo, err := os.Stat(npc.publishPath)
	if err != nil {
		return errorutils.CheckError(err)
	}

	if fileInfo.IsDir() {
		npc.packageInfo, err = npm.ReadPackageInfoFromPackageJson(npc.publishPath)
		return err
	}
	log.Debug("The provided path is not a directory, we assume this is a compressed npm package")
	npc.tarballProvided = true
	npc.packedFilePath = npc.publishPath
	return npc.readPackageInfoFromTarball()
}

func (npc *NpmPublishCommand) readPackageInfoFromTarball() error {
	log.Debug("Extracting info from npm package:", npc.packedFilePath)
	tarball, err := os.Open(npc.packedFilePath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer tarball.Close()
	gZipReader, err := gzip.NewReader(tarball)
	if err != nil {
		return errorutils.CheckError(err)
	}

	tarReader := tar.NewReader(gZipReader)
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return errorutils.CheckError(errors.New("Could not find 'package.json' in the compressed npm package: " + npc.packedFilePath))
			}
			return errorutils.CheckError(err)
		}
		parent := filepath.Dir(hdr.Name)
		if filepath.Base(parent) == "package" && strings.HasSuffix(hdr.Name, "package.json") {
			packageJson, err := ioutil.ReadAll(tarReader)
			if err != nil {
				return errorutils.CheckError(err)
			}

			npc.packageInfo, err = npm.ReadPackageInfo(packageJson)
			return err
		}
	}
}

func deleteCreatedTarballAndError(packedFilePath string, currentError error) error {
	if err := deleteCreatedTarball(packedFilePath); err != nil {
		errorText := fmt.Sprintf("Two errors occurred: \n%s \n%s", currentError, err)
		return errorutils.CheckError(errors.New(errorText))
	}
	return currentError
}

func deleteCreatedTarball(packedFilePath string) error {
	if err := os.Remove(packedFilePath); err != nil {
		return errorutils.CheckError(err)
	}
	log.Debug("Successfully deleted the created npm package:", packedFilePath)
	return nil
}
