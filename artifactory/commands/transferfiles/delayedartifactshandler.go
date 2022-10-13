package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

var maxDelayedArtifactsInFile = 50000

// TransferDelayedArtifactsMng takes care of the multi-threaded-writing of artifacts to be transferred, while maintaining the correct order of the deployment.
// This is needed because, for example, for maven repositories, pom file should be deployed last.
type TransferDelayedArtifactsMng struct {
	// All go routines will write delayedArtifacts to the same channel
	delayedArtifactsChannelMng *DelayedArtifactsChannelMng
	// Writer
	repoKey        string
	phaseStartTime string
	delayedWriter  *SplitContentWriter
}

// Create transfer delays directory inside the JFrog CLI home directory.
func initTransferDelaysDir() error {
	// Create transfer directory (if it doesn't exist)
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	err = fileutils.CreateDirIfNotExist(transferDir)
	if err != nil {
		return err
	}
	// Create delays directory
	delaysDirPath, err := coreutils.GetJfrogTransferDelaysDir()
	if err != nil {
		return err
	}
	return fileutils.CreateDirIfNotExist(delaysDirPath)
}

// Creates a manager for the files transferring process.
func newTransferDelayedArtifactsToFile(delayedArtifactsChannelMng *DelayedArtifactsChannelMng, repoKey string, phaseStartTime string) (*TransferDelayedArtifactsMng, error) {
	err := initTransferDelaysDir()
	if err != nil {
		return nil, err
	}
	return &TransferDelayedArtifactsMng{delayedArtifactsChannelMng: delayedArtifactsChannelMng, repoKey: repoKey, phaseStartTime: phaseStartTime}, err
}

// Expected error file format: <repoKey>-<phaseStartTime in epoch millisecond>-<fileIndex>.json
var delaysFilesRegexp = regexp.MustCompile(`^(.+)-([0-9]{13})-([0-9]+)\.json$`)

func getDelaysFilePrefix(repoKey string, phaseStartTime string) string {
	return fmt.Sprintf("%s-%s", repoKey, phaseStartTime)
}

func (mng *TransferDelayedArtifactsMng) start() (err error) {
	defer func() {
		e := mng.delayedWriter.Close()
		if e == nil {
			err = errorutils.CheckError(e)
		}
	}()

	delaysDirPath, err := coreutils.GetJfrogTransferDelaysDir()
	if err != nil {
		return err
	}

	mng.delayedWriter = NewSplitContentWriter("delayed_artifacts", maxDelayedArtifactsInFile, delaysDirPath, getDelaysFilePrefix(mng.repoKey, mng.phaseStartTime))

	for file := range mng.delayedArtifactsChannelMng.channel {
		log.Debug(fmt.Sprintf("Delaying the upload of file '%s'. Writing it to be uploaded later...", path.Join(file.Repo, file.Path, file.Name)))
		err := mng.delayedWriter.WriteRecord(file)
		if err != nil {
			return err
		}
	}
	return nil
}

type DelayedArtifactsFile struct {
	DelayedArtifacts []FileRepresentation `json:"delayed_artifacts,omitempty"`
}

// Action for the last phase, Upload all the delayed artifact of a given repo
func ConsumeAllDelayFiles(base phaseBase, addedDelayFiles []string) error {
	filesToConsume, err := getDelayFiles([]string{base.repoKey})
	if err != nil {
		return err
	}
	delayFunctions := getDelayUploadComparisonFunctions(base.repoSummary.PackageType)
	if len(filesToConsume) > 0 && len(delayFunctions) > 0 {
		log.Info("Starting to handle delayed artifacts uploads...")
		err = handleDelayedArtifactsFiles(filesToConsume, base, delayFunctions[1:])
		if err == nil {
			log.Info("Done handling delayed artifacts uploads.")
		}
		return err
	}
	return nil
}

func ConsumeDelayFilesIfNoErrors(phase phaseBase, addedDelayFiles []string) error {
	errCount, err := getRetryErrorCount([]string{phase.repoKey})
	if err != nil {
		return err
	}
	// no errors - we can handle all the delay files up to this point
	if errCount == 0 {
		return ConsumeAllDelayFiles(phase, addedDelayFiles)
	}
	// Delay will be handled later at Phase3, reduce the total of progress of this phase by the amount of artifacts that were delayed
	if len(addedDelayFiles) > 0 {
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
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunkData, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&base, &delayHelper, errorsChannelMng) {
			return nil
		}
		return consumeDelayedArtifactsFiles(pcWrapper, filesToConsume, uploadChunkChan, base, delayHelper, errorsChannelMng)
	}
	delayAction := func(pBase phaseBase, addedDelayFiles []string) error {
		// Remove the first delay comparison function to no longer delay it.
		if len(filesToConsume) > 0 && len(delayUploadComparisonFunctions) > 0 {
			return handleDelayedArtifactsFiles(addedDelayFiles, pBase, delayUploadComparisonFunctions[1:])
		}
		return nil
	}
	return manager.doTransferWithProducerConsumer(action, delayAction)
}

func consumeDelayedArtifactsFiles(pcWrapper *producerConsumerWrapper, filesToConsume []string, uploadChunkChan chan UploadedChunkData, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	defer pcWrapper.notifyIfBuilderFinished(true)
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
		err = os.Remove(filePath)
		if err != nil {
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
	if err != nil {
		return delayedArtifactsFile, errorutils.CheckError(err)
	}
	return delayedArtifactsFile, nil
}

// Gets a list of all delay files from the CLI's cache for a specific repo
func getDelayFiles(repoKeys []string) (filesPaths []string, err error) {

	dirPath, err := coreutils.GetJfrogTransferDelaysDir()
	if err != nil {
		return []string{}, err
	}
	exist, err := fileutils.IsDirExists(dirPath, false)
	if !exist || err != nil {
		return []string{}, err
	}

	files, err := fileutils.ListFiles(dirPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		matchAndGroups := delaysFilesRegexp.FindStringSubmatch(filepath.Base(file))
		// Expecting a match and 3 groups. A total of 4 results.
		if len(matchAndGroups) != 4 {
			log.Error("unexpected delay file file-name:", file)
			continue
		}
		// Append the errors file if the first group matches any of the requested repo keys.
		for _, repoKey := range repoKeys {
			if matchAndGroups[1] == repoKey {
				filesPaths = append(filesPaths, file)
				break
			}
		}
	}
	return
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
	case maven:
		fallthrough
	case gradle:
		fallthrough
	case ivy:
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
func (delayHelper delayUploadHelper) delayUploadIfNecessary(phase phaseBase, file FileRepresentation) (delayed, stopped bool) {
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
	channel chan FileRepresentation
	err     error
}

func (mng DelayedArtifactsChannelMng) add(element FileRepresentation) {
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
	channel := make(chan FileRepresentation, fileWritersChannelSize)
	return DelayedArtifactsChannelMng{channel: channel}
}

type SplitContentWriter struct {
	writer *content.ContentWriter

	arrayKey       string
	maxRecordAllow int
	dirPath        string
	filePrefix     string

	recordCount  int
	fileIndex    int
	ContentFiles []string
}

func NewSplitContentWriter(key string, maxRecordsPerFile int, directoryPath string, prefix string) *SplitContentWriter {
	scw := SplitContentWriter{arrayKey: key, maxRecordAllow: maxRecordsPerFile, dirPath: directoryPath, filePrefix: prefix, ContentFiles: []string{}}
	return &scw
}

// Create new file if needed, writes a record and closes a file if it reached its maxRecord
func (w *SplitContentWriter) WriteRecord(record interface{}) error {
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
	if w.recordCount == w.maxRecordAllow {
		return w.closeCurrentFile()
	}
	return nil
}

func (w *SplitContentWriter) closeCurrentFile() error {
	// clean current file
	if w.writer != nil {
		err := w.writer.Close()
		if err != nil {
			return err
		}
		if w.writer.GetFilePath() != "" {
			fullPath := filepath.Join(w.dirPath, fmt.Sprintf("%s-%d.json", w.filePrefix, w.fileIndex))
			log.Debug(fmt.Sprintf("Saving split content JSON file to: %s.", fullPath))
			err = fileutils.MoveFile(w.writer.GetFilePath(), fullPath)
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Saving file failed! failed moving %s to %s", w.writer.GetFilePath(), fullPath), err)
			}

			w.ContentFiles = append(w.ContentFiles, fullPath)
			w.fileIndex++
		}
	}
	// Reset writer and counter.
	w.recordCount = 0
	w.writer = nil
	return nil
}

func (w *SplitContentWriter) Close() error {
	return w.closeCurrentFile()
}
