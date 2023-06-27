package transferfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

var maxDelayedArtifactsInFile = 50000

// TransferDelayedArtifactsMng takes care of the multi-threaded-writing of artifacts to be transferred, while maintaining the correct order of the deployment.
// This is needed because, for example, for maven repositories, pom file should be deployed last.
type TransferDelayedArtifactsMng struct {
	// All go routines will write delayedArtifacts to the same channel
	delayedArtifactsChannelMng *DelayedArtifactsChannelMng
	// The Information needed to determine the file names created by the writer
	repoKey        string
	phaseStartTime string
	// Writes delayed artifacts from channel to files
	delayedWriter *SplitContentWriter
}

// Create transfer delays directory inside the JFrog CLI home directory.
func initTransferDelaysDir(repoKey string) error {
	// Create transfer directory (if it doesn't exist)
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	if err = fileutils.CreateDirIfNotExist(transferDir); err != nil {
		return err
	}
	// Create delays directory
	delaysDirPath, err := getJfrogTransferRepoDelaysDir(repoKey)
	if err != nil {
		return err
	}
	return fileutils.CreateDirIfNotExist(delaysDirPath)
}

// Creates a manager for the process of transferring delayed files. Delayed files are files that should be transferred at the very end of the transfer process, such as pom.xml and manifest.json files.
func newTransferDelayedArtifactsManager(delayedArtifactsChannelMng *DelayedArtifactsChannelMng, repoKey string, phaseStartTime string) (*TransferDelayedArtifactsMng, error) {
	if err := initTransferDelaysDir(repoKey); err != nil {
		return nil, err
	}
	return &TransferDelayedArtifactsMng{delayedArtifactsChannelMng: delayedArtifactsChannelMng, repoKey: repoKey, phaseStartTime: phaseStartTime}, nil
}

func getDelaysFilePrefix(repoKey string, phaseStartTime string) string {
	return fmt.Sprintf("%s-%s", repoKey, phaseStartTime)
}

func (mng *TransferDelayedArtifactsMng) start() (err error) {
	defer func() {
		if mng.delayedWriter != nil {
			if e := mng.delayedWriter.close(); err == nil {
				err = errorutils.CheckError(e)
			}
		}
	}()

	delaysDirPath, err := getJfrogTransferRepoDelaysDir(mng.repoKey)
	if err != nil {
		return err
	}

	mng.delayedWriter = newSplitContentWriter("delayed_artifacts", maxDelayedArtifactsInFile, delaysDirPath, getDelaysFilePrefix(mng.repoKey, mng.phaseStartTime))

	for file := range mng.delayedArtifactsChannelMng.channel {
		log.Debug(fmt.Sprintf("Delaying the upload of file '%s'. Writing it to be uploaded later...", path.Join(file.Repo, file.Path, file.Name)))
		if err = mng.delayedWriter.writeRecord(file); err != nil {
			return err
		}
	}
	return nil
}

type DelayedArtifactsFile struct {
	DelayedArtifacts []api.FileRepresentation `json:"delayed_artifacts,omitempty"`
}

// Collect all the delayed artifact files that were created up to this point for the repository and transfer their artifacts using handleDelayedArtifactsFiles
func consumeAllDelayFiles(base phaseBase) error {
	filesToConsume, err := getDelayFiles([]string{base.repoKey})
	if err != nil {
		return err
	}
	delayFunctions := getDelayUploadComparisonFunctions(base.repoSummary.PackageType)
	if len(filesToConsume) > 0 && len(delayFunctions) > 0 {
		log.Info("Starting to handle delayed artifacts uploads...")
		if err = handleDelayedArtifactsFiles(filesToConsume, base, delayFunctions[1:]); err == nil {
			log.Info("Done handling delayed artifacts uploads.")
		}
	}
	return err
}

// Call consumeAllDelayFiles only if there are no failed transferred files for the repository up to this point.
// In case failed files exists, we reduce the count of files for the given phase by the amount of delayed artifacts.
func consumeDelayFilesIfNoErrors(phase phaseBase, addedDelayFiles []string) error {
	errCount, err := getRetryErrorCount([]string{phase.repoKey})
	if err != nil {
		return err
	}
	// No errors - we can handle all the delayed files created up to this point.
	if errCount == 0 {
		return consumeAllDelayFiles(phase)
	}
	// There were files which we failed to transferred, and therefore we had error files.
	// Therefore, the delayed files should be handled later, as part of Phase 3. We also reduce the count of files of this phase by the amount of files which were delayed.
	if len(addedDelayFiles) > 0 && phase.progressBar != nil {
		phaseTaskProgressBar := phase.progressBar.phases[phase.phaseId].GetTasksProgressBar()
		oldTotal := phaseTaskProgressBar.GetTotal()
		delayCount, err := countDelayFilesContent(addedDelayFiles)
		if err != nil {
			return err
		}
		phaseTaskProgressBar.SetGeneralProgressTotal(oldTotal - int64(delayCount))
	}
	return nil
}

func countDelayFilesContent(filePaths []string) (int, error) {
	count := 0
	for _, file := range filePaths {
		delayFile, err := readDelayFile(file)
		if err != nil {
			return 0, err
		}
		count += len(delayFile.DelayedArtifacts)
	}
	return count, nil
}

func handleDelayedArtifactsFiles(filesToConsume []string, base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) error {
	manager := newTransferManager(base, delayUploadComparisonFunctions)
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&base, &delayHelper, errorsChannelMng) {
			return nil
		}
		return consumeDelayedArtifactsFiles(pcWrapper, filesToConsume, uploadChunkChan, base, delayHelper, errorsChannelMng)
	}
	delayAction := func(pBase phaseBase, addedDelayFiles []string) error {
		// We call this method as a recursion in order to have inner order base on the comparison function list.
		// Remove the first delay comparison function one by one to no longer delay it until the list is empty.
		if len(filesToConsume) > 0 && len(delayUploadComparisonFunctions) > 0 {
			return handleDelayedArtifactsFiles(addedDelayFiles, pBase, delayUploadComparisonFunctions[1:])
		}
		return nil
	}
	return manager.doTransferWithProducerConsumer(action, delayAction)
}

func consumeDelayedArtifactsFiles(pcWrapper *producerConsumerWrapper, filesToConsume []string, uploadChunkChan chan UploadedChunk, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	for _, filePath := range filesToConsume {
		log.Debug("Handling delayed artifacts file: '" + filePath + "'")
		delayedArtifactsFile, err := readDelayFile(filePath)
		if err != nil {
			return err
		}

		shouldStop, err := uploadByChunks(delayedArtifactsFile.DelayedArtifacts, uploadChunkChan, base, delayHelper, errorsChannelMng, pcWrapper)
		if err != nil || shouldStop {
			return err
		}

		// Remove the file, so it won't be consumed again.
		if err = os.Remove(filePath); err != nil {
			return errorutils.CheckError(err)
		}

		log.Debug("Done handling delayed artifacts file: '" + filePath + "'. Deleting it...")
	}
	return nil
}

// Reads a delay file from a given path, parses and populate a given DelayedArtifactsFile instance with the file information
func readDelayFile(path string) (DelayedArtifactsFile, error) {
	// Stores the errors read from the errors file.
	var delayedArtifactsFile DelayedArtifactsFile

	fContent, err := os.ReadFile(path)
	if err != nil {
		return delayedArtifactsFile, errorutils.CheckError(err)
	}

	err = json.Unmarshal(fContent, &delayedArtifactsFile)
	return delayedArtifactsFile, errorutils.CheckError(err)
}

// Gets a list of all delay files from the CLI's cache for a specific repo
func getDelayFiles(repoKeys []string) (filesPaths []string, err error) {
	return getErrorOrDelayFiles(repoKeys, getJfrogTransferRepoDelaysDir)
}

const (
	maven  = "Maven"
	gradle = "Gradle"
	ivy    = "Ivy"
	docker = "Docker"
	conan  = "Conan"
	nuget  = "NuGet"
	sbt    = "SBT"
)

// A function to determine whether the file deployment should be delayed.
type shouldDelayUpload func(string) bool

// Returns an array of functions to control the order of deployment.
func getDelayUploadComparisonFunctions(packageType string) []shouldDelayUpload {
	switch packageType {
	case maven, gradle, ivy:
		return []shouldDelayUpload{func(fileName string) bool {
			return filepath.Ext(fileName) == ".pom"
		}}
	case docker:
		return []shouldDelayUpload{func(fileName string) bool {
			return fileName == "manifest.json"
		}, func(fileName string) bool {
			return fileName == "list.manifest.json"
		}}
	case conan:
		return []shouldDelayUpload{func(fileName string) bool {
			return fileName == "conanfile.py"
		}, func(fileName string) bool {
			return fileName == "conaninfo.txt"
		}, func(fileName string) bool {
			return fileName == ".timestamp"
		}}
	}
	return []shouldDelayUpload{}
}

type delayUploadHelper struct {
	shouldDelayFunctions       []shouldDelayUpload
	delayedArtifactsChannelMng *DelayedArtifactsChannelMng
}

// Decide whether to delay the deployment of a file by running over the shouldDelayUpload array.
// When there are multiple levels of requirements in the deployment order, the first comparison function in the array can be removed each time in order to no longer delay by that rule.
func (delayHelper delayUploadHelper) delayUploadIfNecessary(phase phaseBase, file api.FileRepresentation) (delayed, stopped bool) {
	for _, shouldDelay := range delayHelper.shouldDelayFunctions {
		if ShouldStop(&phase, &delayHelper, nil) {
			return delayed, true
		}
		if shouldDelay(file.Name) {
			delayed = true
			delayHelper.delayedArtifactsChannelMng.add(file)
		}
	}
	return
}

// DelayedArtifactsChannelMng is used when writing 'delayed artifacts' to a common channel.
// If an error occurs while handling the files - this message is used to stop adding elements to the channel.
type DelayedArtifactsChannelMng struct {
	channel chan api.FileRepresentation
	err     error
}

func (mng DelayedArtifactsChannelMng) add(element api.FileRepresentation) {
	mng.channel <- element
}

func (mng DelayedArtifactsChannelMng) shouldStop() bool {
	// Stop adding elements to the channel if a 'blocking' error occurred in a different go routine.
	return mng.err != nil
}

// Close channel
func (mng DelayedArtifactsChannelMng) close() {
	close(mng.channel)
}

func createdDelayedArtifactsChannelMng() DelayedArtifactsChannelMng {
	channel := make(chan api.FileRepresentation, fileWritersChannelSize)
	return DelayedArtifactsChannelMng{channel: channel}
}

// SplitContentWriter writes to files a single JSON object that holds a list of records added as stream.
// It can limit the amount of records per file and splits the content to several files if needed.
type SplitContentWriter struct {
	writer *content.ContentWriter
	// JSON array key of the object
	arrayKey string
	// Limit for the amount of records allowed per file
	maxRecordsAllowed int
	// The path for the directory that will hold the files of the content
	dirPath string
	// The name for the files that will be generated (a counter will added as a suffix to the files by this writer)
	filePrefix string
	// Counter for the amount of records at the current file
	recordCount int
	// Counter for amount if files generated for the content
	fileIndex int
	// List of all the paths of the files that were generated for the content
	contentFiles []string
}

func newSplitContentWriter(key string, maxRecordsPerFile int, directoryPath string, prefix string) *SplitContentWriter {
	return &SplitContentWriter{arrayKey: key, maxRecordsAllowed: maxRecordsPerFile, dirPath: directoryPath, filePrefix: prefix, contentFiles: []string{}}
}

// Create new file if needed, writes a record and closes a file if it reached its maxRecord
func (w *SplitContentWriter) writeRecord(record interface{}) error {
	// Init the content writer, which is responsible for writing to the current file
	if w.writer == nil {
		writer, err := content.NewContentWriter(w.arrayKey, true, false)
		if err != nil {
			return err
		}
		w.writer = writer
	}
	// Write
	w.writer.Write(record)
	w.recordCount++
	// If file contains maximum number of records - reset for next write
	if w.recordCount == w.maxRecordsAllowed {
		return w.closeCurrentFile()
	}
	return nil
}

func (w *SplitContentWriter) closeCurrentFile() error {
	// Close current file
	if w.writer != nil {
		if err := w.writer.Close(); err != nil {
			return err
		}
		if w.writer.GetFilePath() != "" {
			fullPath, err := getUniqueErrorOrDelayFilePath(w.dirPath, func() string {
				return w.filePrefix
			})
			if err != nil {
				return err
			}
			log.Debug(fmt.Sprintf("Saving split content JSON file to: %s.", fullPath))
			if err := fileutils.MoveFile(w.writer.GetFilePath(), fullPath); err != nil {
				return fmt.Errorf("saving file failed! failed moving %s to %s: %w", w.writer.GetFilePath(), fullPath, err)
			}
			w.contentFiles = append(w.contentFiles, fullPath)
			w.fileIndex++
		}
	}
	// Reset writer and counter.
	w.recordCount = 0
	w.writer = nil
	return nil
}

func (w *SplitContentWriter) close() error {
	return w.closeCurrentFile()
}
