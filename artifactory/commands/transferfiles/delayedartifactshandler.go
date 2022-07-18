package transferfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// Max delayedArtifacts that will be written in a file
	maxDelayedArtifactsInFile = 50000
)

// TransferDelayedArtifactsMng takes care of the multi-threaded-writing of artifacts to be transferred, while maintaining the correct order of the deployment.
// This is needed because, for example, for maven repositories, pom file should be deployed last.
type TransferDelayedArtifactsMng struct {
	// All go routines will write delayedArtifacts to the same channel
	delayedArtifactsChannelMng *DelayedArtifactsChannelMng

	// Files containing delayed artifacts to upload later on.
	filesToConsume []string
	deployedWriter delayedArtifactWriter
}

type delayedArtifactWriter struct {
	writer               *content.ContentWriter
	delayedArtifactCount int
}

// Creates a manager for the files transferring process.
func newTransferDelayedArtifactsToFile(delayedArtifactsChannelMng *DelayedArtifactsChannelMng) *TransferDelayedArtifactsMng {
	return &TransferDelayedArtifactsMng{delayedArtifactsChannelMng: delayedArtifactsChannelMng}
}

func (mng *TransferDelayedArtifactsMng) start() (err error) {
	defer func() {
		if mng.deployedWriter.writer != nil {
			e := mng.deployedWriter.writer.Close()
			if err == nil {
				err = errorutils.CheckError(e)
			}
			if mng.deployedWriter.writer.GetFilePath() != "" {
				mng.filesToConsume = append(mng.filesToConsume, mng.deployedWriter.writer.GetFilePath())
			}
		}
	}()

	for file := range mng.delayedArtifactsChannelMng.channel {
		if mng.deployedWriter.writer == nil {
			// Init the content writer, which is responsible for writing delayed artifacts - This means that delayed artifacts will be deployed later, to maintain the right deployment order.
			writer, err := content.NewContentWriter("delayed_artifacts", true, false)
			if err != nil {
				return err
			}
			mng.deployedWriter = delayedArtifactWriter{writer: writer}
		}

		log.Debug(fmt.Sprintf("Delaying the upload of file '%s'. Writing it to be uploaded later...", path.Join(file.Repo, file.Path, file.Name)))
		mng.deployedWriter.writer.Write(file)
		mng.deployedWriter.delayedArtifactCount++
		// If file contains maximum number of delayedArtifacts - create and write to a new delayedArtifacts file.
		if mng.deployedWriter.delayedArtifactCount == maxDelayedArtifactsInFile {
			err = mng.deployedWriter.writer.Close()
			if err == nil {
				return err
			}
			if mng.deployedWriter.writer.GetFilePath() != "" {
				mng.filesToConsume = append(mng.filesToConsume, mng.deployedWriter.writer.GetFilePath())
			}
			// Reset writer and counter.
			mng.deployedWriter.delayedArtifactCount = 0
			mng.deployedWriter.writer = nil
		}
	}
	return nil
}

type DelayedArtifactsFile struct {
	DelayedArtifacts []FileRepresentation `json:"delayed_artifacts,omitempty"`
}

func handleDelayedArtifactsFiles(filesToConsume []string, base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) error {
	log.Info("Starting to handle delayed artifacts uploads...")
	manager := newTransferManager(base, delayUploadComparisonFunctions)
	action := func(uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&base, &delayHelper, errorsChannelMng) {
			return nil
		}
		return consumeDelayedArtifactsFiles(filesToConsume, uploadTokensChan, base, delayHelper, errorsChannelMng)
	}
	err := manager.doTransferWithSingleProducer(action)
	if err == nil {
		log.Info("Done handling delayed artifacts uploads.")
	}
	return err
}

func consumeDelayedArtifactsFiles(filesToConsume []string, uploadTokensChan chan string, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	for _, filePath := range filesToConsume {
		log.Debug("Handling delayed artifacts file: '" + filePath + "'")
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		var delayedArtifactsFile DelayedArtifactsFile
		err = json.Unmarshal(fileContent, &delayedArtifactsFile)
		if err != nil {
			return errorutils.CheckError(err)
		}

		shouldStop, err := uploadByChunks(delayedArtifactsFile.DelayedArtifacts, uploadTokensChan, base, delayHelper, errorsChannelMng)
		if err != nil || shouldStop {
			return err
		}

		// Remove the file, so it won't be consumed again.
		err = os.Remove(filePath)
		if err != nil {
			return errorutils.CheckError(err)
		}

		if base.progressBar != nil {
			if base.phaseId == 0 {
				err = base.progressBar.IncrementPhaseBy(base.phaseId, len(delayedArtifactsFile.DelayedArtifacts))
				if err != nil {
					return err
				}
			}
		}
		log.Debug("Done handling delayed artifacts file: '" + filePath + "'")
	}
	return nil
}

const (
	maven  = "Maven"
	gradle = "Gradle"
	ivy    = "Ivy"
	docker = "Docker"
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
