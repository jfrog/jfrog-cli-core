package generic

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/jfrog/jfrog-cli-core/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type UploadCommand struct {
	GenericCommand
	uploadConfiguration *utils.UploadConfiguration
	buildConfiguration  *utils.BuildConfiguration
	progress            ioUtils.Progress
}

func NewUploadCommand() *UploadCommand {
	return &UploadCommand{GenericCommand: *NewGenericCommand()}
}

func (uc *UploadCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *UploadCommand {
	uc.buildConfiguration = buildConfiguration
	return uc
}

func (uc *UploadCommand) UploadConfiguration() *utils.UploadConfiguration {
	return uc.uploadConfiguration
}

func (uc *UploadCommand) SetUploadConfiguration(uploadConfiguration *utils.UploadConfiguration) *UploadCommand {
	uc.uploadConfiguration = uploadConfiguration
	return uc
}

func (uc *UploadCommand) SetProgress(progress ioUtils.Progress) {
	uc.progress = progress
}

func (uc *UploadCommand) CommandName() string {
	return "rt_upload"
}

func (uc *UploadCommand) Run() error {
	return uc.upload()
}

// Uploads the artifacts in the specified local path pattern to the specified target path.
// Returns the total number of artifacts successfully uploaded.
func (uc *UploadCommand) upload() error {
	// In case of sync-delete get the user to confirm first, and save the operation timestamp.
	syncDeletesProp := ""
	if !uc.DryRun() && uc.SyncDeletesPath() != "" {
		if !uc.Quiet() && !coreutils.AskYesNo("Sync-deletes may delete some artifacts in Artifactory. Are you sure you want to continue?\n"+
			"You can avoid this confirmation message by adding --quiet to the command.", false) {
			return nil
		}
		timestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
		syncDeletesProp = ";sync.deletes.timestamp=" + timestamp
	}

	// Create Service Manager:
	var err error
	uc.uploadConfiguration.MinChecksumDeploySize, err = getMinChecksumDeploySize()
	if err != nil {
		return err
	}
	rtDetails, err := uc.RtDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	servicesManager, err := utils.CreateUploadServiceManager(rtDetails, uc.uploadConfiguration.Threads, uc.DryRun(), uc.progress)
	if err != nil {
		return err
	}

	addVcsProps := false
	buildProps := ""
	// Build Info Collection:
	isCollectBuildInfo := len(uc.buildConfiguration.BuildName) > 0 && len(uc.buildConfiguration.BuildNumber) > 0
	if isCollectBuildInfo && !uc.DryRun() {
		addVcsProps = true
		if err := utils.SaveBuildGeneralDetails(uc.buildConfiguration.BuildName, uc.buildConfiguration.BuildNumber); err != nil {
			return err
		}
		buildProps, err = utils.CreateBuildProperties(uc.buildConfiguration.BuildName, uc.buildConfiguration.BuildNumber)
		if err != nil {
			return err
		}
	}

	var errorOccurred = false
	var uploadParamsArray []services.UploadParams
	// Create UploadParams for all File-Spec groups.
	for i := 0; i < len(uc.Spec().Files); i++ {
		file := uc.Spec().Get(i)
		file.Props += syncDeletesProp
		uploadParams, err := getUploadParams(file, uc.uploadConfiguration, buildProps, addVcsProps)
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		uploadParamsArray = append(uploadParamsArray, uploadParams)
	}

	// Perform upload.
	// In case of build-info collection or a detailed summary request, we use the upload service which provides results file reader,
	// otherwise we use the upload service which provides only general counters.
	var successCount, failCount int
	var resultsReader *content.ContentReader = nil
	if uc.DetailedSummary() || isCollectBuildInfo {
		resultsReader, successCount, failCount, err = servicesManager.UploadFilesWithResultReader(uploadParamsArray...)
		uc.result.SetReader(resultsReader)
	} else {
		successCount, failCount, err = servicesManager.UploadFiles(uploadParamsArray...)
	}
	if err != nil {
		errorOccurred = true
		log.Error(err)
	}
	result := uc.Result()
	result.SetSuccessCount(successCount)
	result.SetFailCount(failCount)
	// If the 'details summary' was requested, then the reader should not be closed now.
	// It will be closed after it will be used to generate the summary.
	if resultsReader != nil && !uc.DetailedSummary() {
		defer func() {
			resultsReader.Close()
			uc.result.SetReader(nil)
		}()
	}
	if errorOccurred {
		err = errors.New("Upload finished with errors, Please review the logs.")
		return err
	}
	if failCount > 0 {
		return err
	}

	if !uc.DryRun() {
		// Handle sync-deletes
		if uc.SyncDeletesPath() != "" {
			err = uc.handleSyncDeletes(syncDeletesProp)
			if err != nil {
				return err
			}
		}
		// Build Info
		if isCollectBuildInfo {
			err, resultBuildInfo := utils.ReadResultBuildInfo(resultsReader)
			if err != nil {
				return err
			}
			buildArtifacts := convertFileInfoToBuildArtifacts(resultBuildInfo.FilesInfo)
			populateFunc := func(partial *buildinfo.Partial) {
				partial.Artifacts = buildArtifacts
				partial.ModuleId = uc.buildConfiguration.Module
			}
			err = utils.SavePartialBuildInfo(uc.buildConfiguration.BuildName, uc.buildConfiguration.BuildNumber, populateFunc)
		}
	}
	return err
}

func convertFileInfoToBuildArtifacts(filesInfo []clientutils.FileInfo) []buildinfo.Artifact {
	buildArtifacts := make([]buildinfo.Artifact, len(filesInfo))
	for i, fileInfo := range filesInfo {
		buildArtifacts[i] = fileInfo.ToBuildArtifacts()
	}
	return buildArtifacts
}

func getMinChecksumDeploySize() (int64, error) {
	minChecksumDeploySize := os.Getenv("JFROG_CLI_MIN_CHECKSUM_DEPLOY_SIZE_KB")
	if minChecksumDeploySize == "" {
		return 10240, nil
	}
	minSize, err := strconv.ParseInt(minChecksumDeploySize, 10, 64)
	err = errorutils.CheckError(err)
	if err != nil {
		return 0, err
	}
	return minSize * 1000, nil
}

func getUploadParams(f *spec.File, configuration *utils.UploadConfiguration, bulidProps string, addVcsProps bool) (uploadParams services.UploadParams, err error) {
	uploadParams = services.NewUploadParams()
	uploadParams.ArtifactoryCommonParams = f.ToArtifactoryCommonParams()
	uploadParams.Deb = configuration.Deb
	uploadParams.Symlink = configuration.Symlink
	uploadParams.MinChecksumDeploy = configuration.MinChecksumDeploySize
	uploadParams.Retries = configuration.Retries
	uploadParams.AddVcsProps = addVcsProps
	uploadParams.BuildProps = bulidProps

	uploadParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}

	uploadParams.Regexp, err = f.IsRegexp(false)
	if err != nil {
		return
	}

	uploadParams.IncludeDirs, err = f.IsIncludeDirs(false)
	if err != nil {
		return
	}

	uploadParams.Flat, err = f.IsFlat(true)
	if err != nil {
		return
	}

	uploadParams.ExplodeArchive, err = f.IsExplode(false)
	if err != nil {
		return
	}

	return
}

func (uc *UploadCommand) handleSyncDeletes(syncDeletesProp string) error {
	servicesManager, err := utils.CreateServiceManager(uc.rtDetails, false)
	if err != nil {
		return err
	}
	deleteSpec := createDeleteSpecForSync(uc.SyncDeletesPath(), syncDeletesProp)
	deleteParams, err := getDeleteParams(deleteSpec.Get(0))
	if err != nil {
		return err
	}
	resultItems, err := servicesManager.GetPathsToDelete(deleteParams)
	if err != nil {
		return err
	}
	defer resultItems.Close()
	_, err = servicesManager.DeleteFiles(resultItems)
	return err
}

func createDeleteSpecForSync(deletePattern string, syncDeletesProp string) *spec.SpecFiles {
	return spec.NewBuilder().
		Pattern(deletePattern).
		ExcludeProps(syncDeletesProp).
		Recursive(true).
		BuildSpec()
}
