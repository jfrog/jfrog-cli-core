package generic

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	buildinfo "github.com/jfrog/build-info-go/entities"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type DownloadCommand struct {
	buildConfiguration *utils.BuildConfiguration
	GenericCommand
	configuration *utils.DownloadConfiguration
	progress      ioUtils.ProgressMgr
}

func NewDownloadCommand() *DownloadCommand {
	return &DownloadCommand{GenericCommand: *NewGenericCommand()}
}

func (dc *DownloadCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *DownloadCommand {
	dc.buildConfiguration = buildConfiguration
	return dc
}

func (dc *DownloadCommand) Configuration() *utils.DownloadConfiguration {
	return dc.configuration
}

func (dc *DownloadCommand) SetConfiguration(configuration *utils.DownloadConfiguration) *DownloadCommand {
	dc.configuration = configuration
	return dc
}

func (dc *DownloadCommand) SetProgress(progress ioUtils.ProgressMgr) {
	dc.progress = progress
}

func (dc *DownloadCommand) ShouldPrompt() bool {
	return !dc.DryRun() && dc.SyncDeletesPath() != "" && !dc.Quiet()
}

func (dc *DownloadCommand) CommandName() string {
	return "rt_download"
}

func (dc *DownloadCommand) Run() error {
	return dc.download()
}

func (dc *DownloadCommand) download() error {
	// Init progress bar if needed
	if dc.progress != nil {
		dc.progress.InitProgressReaders()
	}
	// Create Service Manager:
	servicesManager, err := utils.CreateDownloadServiceManager(dc.serverDetails, dc.configuration.Threads, dc.retries, dc.DryRun(), dc.progress)
	if err != nil {
		return err
	}

	// Build Info Collection:
	toCollect, err := dc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}
	if toCollect && !dc.DryRun() {
		buildName, err := dc.buildConfiguration.GetBuildName()
		if err != nil {
			return err
		}
		buildNumber, err := dc.buildConfiguration.GetBuildNumber()
		if err != nil {
			return err
		}
		if err = utils.SaveBuildGeneralDetails(buildName, buildNumber, dc.buildConfiguration.GetProject()); err != nil {
			return err
		}
	}

	var errorOccurred = false
	var downloadParamsArray []services.DownloadParams
	// Create DownloadParams for all File-Spec groups.
	for i := 0; i < len(dc.Spec().Files); i++ {
		downParams, err := getDownloadParams(dc.Spec().Get(i), dc.configuration)
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		downloadParamsArray = append(downloadParamsArray, downParams)
	}
	// Perform download.
	// In case of build-info collection/sync-deletes operation/a detailed summary is required, we use the download service which provides results file reader,
	// otherwise we use the download service which provides only general counters.
	var totalDownloaded, totalFailed int
	var summary *serviceutils.OperationSummary
	if toCollect || dc.SyncDeletesPath() != "" || dc.DetailedSummary() {
		summary, err = servicesManager.DownloadFilesWithSummary(downloadParamsArray...)
		if err != nil {
			errorOccurred = true
			log.Error(err)
		}
		if summary != nil {
			defer summary.ArtifactsDetailsReader.Close()
			// If 'detailed summary' was requested, then the reader should not be closed here.
			// It will be closed after it will be used to generate the summary.
			if dc.DetailedSummary() {
				dc.result.SetReader(summary.TransferDetailsReader)
			} else {
				defer summary.TransferDetailsReader.Close()
			}
			totalDownloaded = summary.TotalSucceeded
			totalFailed = summary.TotalFailed
		}
	} else {
		totalDownloaded, totalFailed, err = servicesManager.DownloadFiles(downloadParamsArray...)
		if err != nil {
			errorOccurred = true
			log.Error(err)
		}
	}
	dc.result.SetSuccessCount(totalDownloaded)
	dc.result.SetFailCount(totalFailed)
	// Check for errors.
	if errorOccurred {
		return errors.New("Download finished with errors, please review the logs.")
	}
	if dc.DryRun() {
		dc.result.SetSuccessCount(totalDownloaded)
		dc.result.SetFailCount(0)
		return err
	} else if dc.SyncDeletesPath() != "" {
		absSyncDeletesPath, err := filepath.Abs(dc.SyncDeletesPath())
		if err != nil {
			return errorutils.CheckError(err)
		}
		if _, err = os.Stat(absSyncDeletesPath); err == nil {
			// Unmarshal the local paths of the downloaded files from the results file reader
			tmpRoot, err := createDownloadResultEmptyTmpReflection(summary.TransferDetailsReader)
			defer fileutils.RemoveTempDir(tmpRoot)
			if err != nil {
				return err
			}
			walkFn := createSyncDeletesWalkFunction(tmpRoot)
			err = fileutils.Walk(dc.SyncDeletesPath(), walkFn, false)
			if err != nil {
				return errorutils.CheckError(err)
			}
		} else if os.IsNotExist(err) {
			log.Info("Sync-deletes path", absSyncDeletesPath, "does not exists.")
		}
	}
	log.Debug("Downloaded", strconv.Itoa(totalDownloaded), "artifacts.")

	// Build Info
	if toCollect {
		buildName, err := dc.buildConfiguration.GetBuildName()
		if err != nil {
			return err
		}
		buildNumber, err := dc.buildConfiguration.GetBuildNumber()
		if err != nil {
			return err
		}
		var buildDependencies []buildinfo.Dependency
		buildDependencies, err = serviceutils.ConvertArtifactsDetailsToBuildInfoDependencies(summary.ArtifactsDetailsReader)
		if err != nil {
			return err
		}
		populateFunc := func(partial *buildinfo.Partial) {
			partial.Dependencies = buildDependencies
			partial.ModuleId = dc.buildConfiguration.GetModule()
			partial.ModuleType = buildinfo.Generic
		}
		err = utils.SavePartialBuildInfo(buildName, buildNumber, dc.buildConfiguration.GetProject(), populateFunc)
	}

	return err
}

func getDownloadParams(f *spec.File, configuration *utils.DownloadConfiguration) (downParams services.DownloadParams, err error) {
	downParams = services.NewDownloadParams()
	downParams.CommonParams, err = f.ToCommonParams()
	if err != nil {
		return
	}
	downParams.Symlink = configuration.Symlink
	downParams.MinSplitSize = configuration.MinSplitSize
	downParams.SplitCount = configuration.SplitCount

	downParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}

	downParams.IncludeDirs, err = f.IsIncludeDirs(false)
	if err != nil {
		return
	}

	downParams.Flat, err = f.IsFlat(false)
	if err != nil {
		return
	}

	downParams.Explode, err = f.IsExplode(false)
	if err != nil {
		return
	}

	downParams.ValidateSymlink, err = f.IsVlidateSymlinks(false)
	if err != nil {
		return
	}

	downParams.ExcludeArtifacts, err = f.IsExcludeArtifacts(false)
	if err != nil {
		return
	}

	downParams.IncludeDeps, err = f.IsIncludeDeps(false)
	if err != nil {
		return
	}

	downParams.Transitive = strings.ToLower(os.Getenv(coreutils.TransitiveDownload)) == "true"

	downParams.PublicGpgKey = f.GetPublicGpgKey()

	return
}

// We will create the same downloaded hierarchies under a temp directory with 0-size files.
// We will use this "empty reflection" of the download operation to determine whether a file was downloaded or not while walking the real filesystem from sync-deletes root.
func createDownloadResultEmptyTmpReflection(reader *content.ContentReader) (tmpRoot string, err error) {
	tmpRoot, err = fileutils.CreateTempDir()
	if errorutils.CheckError(err) != nil {
		return
	}
	for path := new(clientutils.FileTransferDetails); reader.NextRecord(path) == nil; path = new(clientutils.FileTransferDetails) {
		var absDownloadPath string
		absDownloadPath, err = filepath.Abs(path.TargetPath)
		if errorutils.CheckError(err) != nil {
			return
		}
		legalPath := createLegalPath(tmpRoot, absDownloadPath)
		tmpFileRoot := filepath.Dir(legalPath)
		err = os.MkdirAll(tmpFileRoot, os.ModePerm)
		if errorutils.CheckError(err) != nil {
			return
		}
		var tmpFile *os.File
		tmpFile, err = os.Create(legalPath)
		if errorutils.CheckError(err) != nil {
			return
		}
		err = tmpFile.Close()
		if errorutils.CheckError(err) != nil {
			return
		}
	}
	return
}

// Creates absolute path for temp file suitable for all environments
func createLegalPath(root, path string) string {
	// Avoid concatenating the volume name (e.g "C://") in Windows environment.
	volumeName := filepath.VolumeName(path)
	if volumeName != "" && strings.HasPrefix(path, volumeName) {
		alternativeVolumeName := "VolumeName" + string(volumeName[0])
		path = strings.Replace(path, volumeName, alternativeVolumeName, 1)
	}
	// Join the current path to the temp root provided.
	path = filepath.Join(root, path)
	return path
}

func createSyncDeletesWalkFunction(tempRoot string) fileutils.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		// Convert path to absolute path
		path, err = filepath.Abs(path)
		if errorutils.CheckError(err) != nil {
			return err
		}
		pathToCheck := createLegalPath(tempRoot, path)

		// If the path exists under the temp root directory, it means it's been downloaded during the last operations, and cannot be deleted.
		if fileutils.IsPathExists(pathToCheck, false) {
			return nil
		}
		log.Info("Deleting:", path)
		if info.IsDir() {
			// If current path is a dir - remove all content and return SkipDir to stop walking this path
			err = os.RemoveAll(path)
			if err == nil {
				return fileutils.SkipDir
			}
		} else {
			// Path is a file
			err = os.Remove(path)
		}

		return errorutils.CheckError(err)
	}
}
