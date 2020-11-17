package generic

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
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
	progress      ioUtils.Progress
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

func (dc *DownloadCommand) SetProgress(progress ioUtils.Progress) {
	dc.progress = progress
}

func (dc *DownloadCommand) CommandName() string {
	return "rt_download"
}

func (dc *DownloadCommand) Run() error {
	return dc.download()
}

func (dc *DownloadCommand) download() error {
	if dc.SyncDeletesPath() != "" && !dc.Quiet() && !coreutils.AskYesNo("Sync-deletes may delete some files in your local file system. Are you sure you want to continue?\n"+
		"You can avoid this confirmation message by adding --quiet to the command.", false) {
		return nil
	}

	// Create Service Manager:
	servicesManager, err := utils.CreateDownloadServiceManager(dc.rtDetails, dc.configuration.Threads, dc.DryRun(), dc.progress)
	if err != nil {
		return err
	}

	// Build Info Collection:
	isCollectBuildInfo := len(dc.buildConfiguration.BuildName) > 0 && len(dc.buildConfiguration.BuildNumber) > 0
	if isCollectBuildInfo && !dc.DryRun() {
		if err = utils.SaveBuildGeneralDetails(dc.buildConfiguration.BuildName, dc.buildConfiguration.BuildNumber); err != nil {
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
	var totalDownloaded, totalExpected int
	var resultsReader *content.ContentReader = nil
	if isCollectBuildInfo || dc.SyncDeletesPath() != "" || dc.DetailedSummary() {
		resultsReader, totalDownloaded, totalExpected, err = servicesManager.DownloadFilesWithResultReader(downloadParamsArray...)
		dc.result.SetReader(resultsReader)
	} else {
		totalDownloaded, totalExpected, err = servicesManager.DownloadFiles(downloadParamsArray...)
	}
	if err != nil {
		errorOccurred = true
		log.Error(err)
	}
	dc.result.SetSuccessCount(totalDownloaded)
	dc.result.SetFailCount(totalExpected - totalDownloaded)
	// If the 'details summary' was requested, then the reader should not be closed now.
	// It will be closed after it will be used to generate the summary.
	if resultsReader != nil && !dc.DetailedSummary() {
		defer func() {
			resultsReader.Close()
			dc.result.SetReader(nil)
		}()
	}
	// Check for errors.
	if errorOccurred {
		return errors.New("Download finished with errors, please review the logs.")
	}
	if dc.DryRun() {
		dc.result.SetSuccessCount(totalExpected)
		dc.result.SetFailCount(0)
		return err
	} else if dc.SyncDeletesPath() != "" {
		absSyncDeletesPath, err := filepath.Abs(dc.SyncDeletesPath())
		if err != nil {
			return errorutils.CheckError(err)
		}
		if _, err = os.Stat(absSyncDeletesPath); err == nil {
			// Unmarshal the local paths of the downloaded files from the results file reader
			tmpRoot, err := createDownloadResultEmptyTmpReflection(resultsReader)
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
	if isCollectBuildInfo {
		err, resultBuildInfo := utils.ReadResultBuildInfo(resultsReader)
		if err != nil {
			return err
		}
		buildDependencies := convertFileInfoToBuildDependencies(resultBuildInfo.FilesInfo)
		populateFunc := func(partial *buildinfo.Partial) {
			partial.Dependencies = buildDependencies
			partial.ModuleId = dc.buildConfiguration.Module
		}
		err = utils.SavePartialBuildInfo(dc.buildConfiguration.BuildName, dc.buildConfiguration.BuildNumber, populateFunc)
	}

	return err
}

func convertFileInfoToBuildDependencies(filesInfo []clientutils.FileInfo) []buildinfo.Dependency {
	buildDependencies := make([]buildinfo.Dependency, len(filesInfo))
	for i, fileInfo := range filesInfo {
		dependency := buildinfo.Dependency{Checksum: &buildinfo.Checksum{}}
		dependency.Md5 = fileInfo.Md5
		dependency.Sha1 = fileInfo.Sha1
		// Artifact name in build info as the name in artifactory
		filename, _ := fileutils.GetFileAndDirFromPath(fileInfo.ArtifactoryPath)
		dependency.Id = filename
		buildDependencies[i] = dependency
	}
	return buildDependencies
}

func getDownloadParams(f *spec.File, configuration *utils.DownloadConfiguration) (downParams services.DownloadParams, err error) {
	downParams = services.NewDownloadParams()
	downParams.ArtifactoryCommonParams = f.ToArtifactoryCommonParams()
	downParams.Symlink = configuration.Symlink
	downParams.MinSplitSize = configuration.MinSplitSize
	downParams.SplitCount = configuration.SplitCount
	downParams.Retries = configuration.Retries

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

	return
}

// We will create the same downloaded hierarchies under a temp directory with 0-size files.
// We will use this "empty reflection" of the download operation to determine whether a file was downloaded or not while walking the real filesystem from sync-deletes root.
func createDownloadResultEmptyTmpReflection(reader *content.ContentReader) (tmpRoot string, err error) {
	tmpRoot, err = fileutils.CreateTempDir()
	if errorutils.CheckError(err) != nil {
		return
	}
	for path := new(localPath); reader.NextRecord(path) == nil; path = new(localPath) {
		var absDownloadPath string
		absDownloadPath, err = filepath.Abs(path.LocalPath)
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

type localPath struct {
	LocalPath string `json:"localPath,omitempty"`
}
