package generic

import (
	"errors"

	buildInfo "github.com/jfrog/build-info-go/entities"

	ioutils "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	rtServicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strconv"
	"time"
)

type UploadCommand struct {
	GenericCommand
	uploadConfiguration *utils.UploadConfiguration
	buildConfiguration  *build.BuildConfiguration
	progress            ioUtils.ProgressMgr
}

func NewUploadCommand() *UploadCommand {
	return &UploadCommand{GenericCommand: *NewGenericCommand()}
}

func (uc *UploadCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *UploadCommand {
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

func (uc *UploadCommand) SetProgress(progress ioUtils.ProgressMgr) {
	uc.progress = progress
}

func (uc *UploadCommand) ShouldPrompt() bool {
	return uc.syncDelete() && !uc.Quiet()
}

func (uc *UploadCommand) syncDelete() bool {
	return !uc.DryRun() && uc.SyncDeletesPath() != ""
}

func (uc *UploadCommand) CommandName() string {
	return "rt_upload"
}

func (uc *UploadCommand) Run() error {
	return uc.upload()
}

// Uploads the artifacts in the specified local path pattern to the specified target path.
// Returns the total number of artifacts successfully uploaded.
func (uc *UploadCommand) upload() (err error) {
	// Init progress bar if needed
	if uc.progress != nil {
		uc.progress.InitProgressReaders()
	}
	// In case of sync-delete get the user to confirm first, and save the operation timestamp.
	syncDeletesProp := ""
	if uc.syncDelete() {
		timestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
		syncDeletesProp = "sync.deletes.timestamp=" + timestamp
	}

	// Create Service Manager:
	uc.uploadConfiguration.MinChecksumDeploySize, err = utils.GetMinChecksumDeploySize()
	if err != nil {
		return
	}
	serverDetails, err := uc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return
	}
	servicesManager, err := utils.CreateUploadServiceManager(serverDetails, uc.uploadConfiguration.Threads, uc.retries, uc.retryWaitTimeMilliSecs, uc.DryRun(), uc.progress)
	if err != nil {
		return
	}

	addVcsProps := false
	buildProps := ""
	// Build Info Collection:
	toCollect, err := uc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return
	}
	if toCollect && !uc.DryRun() {
		addVcsProps = true
		buildProps, err = build.CreateBuildPropsFromConfiguration(uc.buildConfiguration)
		if err != nil {
			return err
		}
	}

	var errorOccurred = false
	var uploadParamsArray []services.UploadParams
	// Create UploadParams for all File-Spec groups.
	for i := 0; i < len(uc.Spec().Files); i++ {
		file := uc.Spec().Get(i)
		file.TargetProps = clientUtils.AddProps(file.TargetProps, file.Props)
		file.TargetProps = clientUtils.AddProps(file.TargetProps, syncDeletesProp)
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
	var artifactsDetailsReader *content.ContentReader = nil
	if uc.DetailedSummary() || toCollect {
		var summary *rtServicesUtils.OperationSummary
		summary, err = servicesManager.UploadFilesWithSummary(uploadParamsArray...)
		if err != nil {
			errorOccurred = true
			log.Error(err)
		}
		if summary != nil {
			artifactsDetailsReader = summary.ArtifactsDetailsReader
			defer ioutils.Close(artifactsDetailsReader, &err)
			// If 'detailed summary' was requested, then the reader should not be closed here.
			// It will be closed after it will be used to generate the summary.
			if uc.DetailedSummary() {
				uc.result.SetReader(summary.TransferDetailsReader)
			} else {
				err = summary.TransferDetailsReader.Close()
				if err != nil {
					errorOccurred = true
					log.Error(err)
				}
			}
			successCount = summary.TotalSucceeded
			failCount = summary.TotalFailed
		}
	} else {
		successCount, failCount, err = servicesManager.UploadFiles(uploadParamsArray...)
		if err != nil {
			errorOccurred = true
			log.Error(err)
		}
	}
	uc.result.SetSuccessCount(successCount)
	uc.result.SetFailCount(failCount)
	if errorOccurred {
		err = errors.New("upload finished with errors. Review the logs for more information")
		return
	}
	if failCount > 0 {
		return
	}

	// Handle sync-deletes
	if uc.syncDelete() {
		err = uc.handleSyncDeletes(syncDeletesProp)
		if err != nil {
			return
		}
	}

	// Build info
	if !uc.DryRun() && toCollect {
		var buildArtifacts []buildInfo.Artifact
		buildArtifacts, err = rtServicesUtils.ConvertArtifactsDetailsToBuildInfoArtifacts(artifactsDetailsReader)
		if err != nil {
			return
		}
		return build.PopulateBuildArtifactsAsPartials(buildArtifacts, uc.buildConfiguration, buildInfo.Generic)
	}
	return
}

func getUploadParams(f *spec.File, configuration *utils.UploadConfiguration, buildProps string, addVcsProps bool) (uploadParams services.UploadParams, err error) {
	uploadParams = services.NewUploadParams()
	uploadParams.CommonParams, err = f.ToCommonParams()
	if err != nil {
		return
	}
	uploadParams.Deb = configuration.Deb
	uploadParams.MinChecksumDeploy = configuration.MinChecksumDeploySize
	uploadParams.MinSplitSize = configuration.MinSplitSizeMB * rtServicesUtils.SizeMiB
	uploadParams.SplitCount = configuration.SplitCount
	uploadParams.AddVcsProps = addVcsProps
	uploadParams.BuildProps = buildProps
	uploadParams.Archive = f.Archive
	uploadParams.TargetPathInArchive = f.TargetPathInArchive

	uploadParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}

	uploadParams.Regexp, err = f.IsRegexp(false)
	if err != nil {
		return
	}

	uploadParams.Ant, err = f.IsAnt(false)
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

	uploadParams.Symlink, err = f.IsSymlinks(false)
	if err != nil {
		return
	}

	return
}

func (uc *UploadCommand) handleSyncDeletes(syncDeletesProp string) (err error) {
	servicesManager, err := utils.CreateServiceManager(uc.serverDetails, uc.retries, uc.retryWaitTimeMilliSecs, false)
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
	defer ioutils.Close(resultItems, &err)
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
