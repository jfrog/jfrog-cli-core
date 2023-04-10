package npm

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	biutils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/gofrog/version"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// The --pack-destination argument of npm pack was introduced in npm version 7.18.0.
const packDestinationNpmMinVersion = "7.18.0"

type NpmPublishCommandArgs struct {
	NpmCommand
	workingDirectory       string
	packedFilePath         string
	packageInfo            *biutils.PackageInfo
	publishPath            string
	tarballProvided        bool
	artifactsDetailsReader *content.ContentReader
	xrayScan               bool
	scanOutputFormat       xrutils.OutputFormat
}

type NpmPublishCommand struct {
	configFilePath  string
	commandName     string
	result          *commandsutils.Result
	detailedSummary bool
	npmVersion      *version.Version
	*NpmPublishCommandArgs
}

func NewNpmPublishCommand() *NpmPublishCommand {
	return &NpmPublishCommand{NpmPublishCommandArgs: NewNpmPublishCommandArgs(), commandName: "rt_npm_publish", result: new(commandsutils.Result)}
}

func NewNpmPublishCommandArgs() *NpmPublishCommandArgs {
	return &NpmPublishCommandArgs{}
}

func (npc *NpmPublishCommand) ServerDetails() (*config.ServerDetails, error) {
	return npc.serverDetails, nil
}

func (npc *NpmPublishCommand) SetConfigFilePath(configFilePath string) *NpmPublishCommand {
	npc.configFilePath = configFilePath
	return npc
}

func (npc *NpmPublishCommand) SetArgs(args []string) *NpmPublishCommand {
	npc.NpmPublishCommandArgs.npmArgs = args
	return npc
}

func (npc *NpmPublishCommand) SetDetailedSummary(detailedSummary bool) *NpmPublishCommand {
	npc.detailedSummary = detailedSummary
	return npc
}

func (npc *NpmPublishCommand) SetXrayScan(xrayScan bool) *NpmPublishCommand {
	npc.xrayScan = xrayScan
	return npc
}

func (npc *NpmPublishCommand) SetScanOutputFormat(format xrutils.OutputFormat) *NpmPublishCommand {
	npc.scanOutputFormat = format
	return npc
}

func (npc *NpmPublishCommand) Result() *commandsutils.Result {
	return npc.result
}

func (npc *NpmPublishCommand) IsDetailedSummary() bool {
	return npc.detailedSummary
}

func (npc *NpmPublishCommand) Init() error {
	var err error
	npc.npmVersion, npc.executablePath, err = biutils.GetNpmVersionAndExecPath(log.Logger)
	if err != nil {
		return err
	}
	detailedSummary, xrayScan, scanOutputFormat, filteredNpmArgs, buildConfiguration, err := commandsutils.ExtractNpmOptionsFromArgs(npc.NpmPublishCommandArgs.npmArgs)
	if err != nil {
		return err
	}
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
		rtDetails, err := deployerParams.ServerDetails()
		if err != nil {
			return errorutils.CheckError(err)
		}
		npc.SetBuildConfiguration(buildConfiguration).SetRepo(deployerParams.TargetRepo()).SetNpmArgs(filteredNpmArgs).SetServerDetails(rtDetails)
	}
	npc.SetDetailedSummary(detailedSummary).SetXrayScan(xrayScan).SetScanOutputFormat(scanOutputFormat)
	return nil
}

func (npc *NpmPublishCommand) Run() (err error) {
	log.Info("Running npm Publish")
	err = npc.preparePrerequisites()
	if err != nil {
		return err
	}

	var npmBuild *build.Build
	var buildName, buildNumber, project string
	if npc.collectBuildInfo {
		buildName, err = npc.buildConfiguration.GetBuildName()
		if err != nil {
			return err
		}
		buildNumber, err = npc.buildConfiguration.GetBuildNumber()
		if err != nil {
			return err
		}
		project = npc.buildConfiguration.GetProject()
		buildInfoService := utils.CreateBuildInfoService()
		npmBuild, err = buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, project)
		if err != nil {
			return errorutils.CheckError(err)
		}
	}

	if !npc.tarballProvided {
		if err := npc.pack(); err != nil {
			return err
		}
	}

	if err := npc.publish(); err != nil {
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

	npmModule, err := npmBuild.AddNpmModule("")
	if err != nil {
		return errorutils.CheckError(err)
	}
	if npc.buildConfiguration.GetModule() != "" {
		npmModule.SetName(npc.buildConfiguration.GetModule())
	}
	buildArtifacts, err := specutils.ConvertArtifactsDetailsToBuildInfoArtifacts(npc.artifactsDetailsReader)
	if err != nil {
		return err
	}
	defer func() {
		e := npc.artifactsDetailsReader.Close()
		if err == nil {
			err = e
		}
	}()
	err = npmModule.AddArtifacts(buildArtifacts...)
	if err != nil {
		return errorutils.CheckError(err)
	}

	log.Info("npm publish finished successfully.")
	return nil
}

func (npc *NpmPublishCommand) CommandName() string {
	return npc.commandName
}

func (npc *NpmPublishCommand) preparePrerequisites() error {
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
	npc.collectBuildInfo, err = npc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}
	if err = npc.setPublishPath(); err != nil {
		return err
	}

	artDetails, err := npc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}

	if err = utils.ValidateRepoExists(npc.repo, artDetails); err != nil {
		return err
	}

	return npc.setPackageInfo()
}

func (npc *NpmPublishCommand) pack() error {
	log.Debug("Creating npm package.")
	packageFileName, err := npm.Pack(npc.npmArgs, npc.executablePath)
	if err != nil {
		return err
	}

	tarballDir, err := npc.getTarballDir()
	if err != nil {
		return err
	}

	npc.packedFilePath = filepath.Join(tarballDir, packageFileName)
	log.Debug("Created npm package at", npc.packedFilePath)
	return nil
}

func (npc *NpmPublishCommand) getTarballDir() (string, error) {
	if npc.npmVersion == nil || npc.npmVersion.Compare(packDestinationNpmMinVersion) > 0 {
		return npc.workingDirectory, nil
	}

	// Extract pack destination argument from the args.
	flagIndex, _, dest, err := coreutils.FindFlag("--pack-destination", npc.NpmPublishCommandArgs.npmArgs)
	if err != nil || flagIndex == -1 {
		return npc.workingDirectory, err
	}
	return dest, nil
}

func (npc *NpmPublishCommand) publish() error {
	log.Debug("Deploying npm package.")
	if err := npc.readPackageInfoFromTarball(); err != nil {
		return err
	}
	target := fmt.Sprintf("%s/%s", npc.repo, npc.packageInfo.GetDeployPath())

	// If requested, perform a Xray binary scan before deployment. If a FailBuildError is returned, skip the deployment.
	if npc.xrayScan {
		fileSpec := spec.NewBuilder().
			Pattern(npc.packedFilePath).
			Target(npc.repo + "/").
			BuildSpec()
		err := commandsutils.ConditionalUploadScanFunc(npc.serverDetails, fileSpec, 1, npc.scanOutputFormat)
		if err != nil {
			return err
		}
	}
	return npc.doDeploy(target, npc.serverDetails)
}

func (npc *NpmPublishCommand) doDeploy(target string, artDetails *config.ServerDetails) error {
	servicesManager, err := utils.CreateServiceManager(artDetails, -1, 0, false)
	if err != nil {
		return err
	}
	up := services.NewUploadParams()
	up.CommonParams = &specutils.CommonParams{Pattern: npc.packedFilePath, Target: target}
	var totalFailed int
	if npc.collectBuildInfo || npc.detailedSummary {
		if npc.collectBuildInfo {
			buildName, err := npc.buildConfiguration.GetBuildName()
			if err != nil {
				return err
			}
			buildNumber, err := npc.buildConfiguration.GetBuildNumber()
			if err != nil {
				return err
			}
			err = utils.SaveBuildGeneralDetails(buildName, buildNumber, npc.buildConfiguration.GetProject())
			if err != nil {
				return err
			}
			up.BuildProps, err = utils.CreateBuildProperties(buildName, buildNumber, npc.buildConfiguration.GetProject())
			if err != nil {
				return err
			}
		}
		summary, err := servicesManager.UploadFilesWithSummary(up)
		if err != nil {
			return err
		}
		totalFailed = summary.TotalFailed
		if npc.collectBuildInfo {
			npc.artifactsDetailsReader = summary.ArtifactsDetailsReader
		} else {
			err = summary.ArtifactsDetailsReader.Close()
			if err != nil {
				return err
			}
		}
		if npc.detailedSummary {
			npc.result.SetReader(summary.TransferDetailsReader)
			npc.result.SetFailCount(totalFailed)
			npc.result.SetSuccessCount(summary.TotalSucceeded)
		} else {
			err = summary.TransferDetailsReader.Close()
			if err != nil {
				return err
			}
		}
	} else {
		_, totalFailed, err = servicesManager.UploadFiles(up)
		if err != nil {
			return err
		}
	}

	// We are deploying only one Artifact which have to be deployed, in case of failure we should fail
	if totalFailed > 0 {
		return errorutils.CheckErrorf("Failed to upload the npm package to Artifactory. See Artifactory logs for more details.")
	}
	return nil
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
		npc.packageInfo, err = biutils.ReadPackageInfoFromPackageJson(npc.publishPath, npc.npmVersion)
		return err
	}
	log.Debug("The provided path is not a directory, we assume this is a compressed npm package")
	npc.tarballProvided = true
	npc.packedFilePath = npc.publishPath
	return npc.readPackageInfoFromTarball()
}

func (npc *NpmPublishCommand) readPackageInfoFromTarball() (err error) {
	log.Debug("Extracting info from npm package:", npc.packedFilePath)
	tarball, err := os.Open(npc.packedFilePath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := tarball.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	gZipReader, err := gzip.NewReader(tarball)
	if err != nil {
		return errorutils.CheckError(err)
	}

	tarReader := tar.NewReader(gZipReader)
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return errorutils.CheckErrorf("Could not find 'package.json' in the compressed npm package: " + npc.packedFilePath)
			}
			return errorutils.CheckError(err)
		}
		if hdr.Name == "package/package.json" {
			packageJson, err := io.ReadAll(tarReader)
			if err != nil {
				return errorutils.CheckError(err)
			}

			npc.packageInfo, err = biutils.ReadPackageInfo(packageJson, npc.npmVersion)
			return err
		}
	}
}

func deleteCreatedTarballAndError(packedFilePath string, currentError error) error {
	if err := deleteCreatedTarball(packedFilePath); err != nil {
		errorText := fmt.Sprintf("Two errors occurred: \n%s \n%s", currentError, err)
		return errorutils.CheckErrorf(errorText)
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
