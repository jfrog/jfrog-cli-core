package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

const (
	// Max delayedArtifacts that will be written in a file
	maxDelayedArtifactsInFile = 50000
)

// TransferDelayedArtifactsMng managing multi threads writing delayed deployment artifacts in order to keep the order of deployment.
type TransferDelayedArtifactsMng struct {
	// All go routines will write delayedArtifacts to this channel
	delayedArtifactsChannel chan FileRepresentation

	// Files containing delayed artifacts to upload later on.
	filesToConsume []string
	deployedWriter delayedArtifactWriter
}

type delayedArtifactWriter struct {
	writer               *content.ContentWriter
	delayedArtifactCount int
	// In case we have multiple delayedArtifacts files - we index them
	fileIndex int
}

func (mng *TransferDelayedArtifactsMng) initDelayedArtifactWriter() (err error) {
	// Init the content writer which responsible for writing delayed artifacts - That means delayed artifacts that we should upload later, in a certain order.
	writer, err := mng.newContentWriter()
	if err != nil {
		return err
	}
	mng.deployedWriter = delayedArtifactWriter{writer: writer, fileIndex: 0}
	return nil
}

// Creates a manager for the files transferring process.
func newTransferDelayedArtifactsToFile(delayedArtifactsChannel chan FileRepresentation) (*TransferDelayedArtifactsMng, error) {
	mng := TransferDelayedArtifactsMng{delayedArtifactsChannel: delayedArtifactsChannel}
	err := mng.initDelayedArtifactWriter()
	return &mng, err
}

func (mng *TransferDelayedArtifactsMng) start() error {
	for file := range mng.delayedArtifactsChannel {
		err := mng.writeDelayedArtifactContent(file)
		if err != nil {
			return err
		}
	}
	return mng.closeWriter()
}

func (mng *TransferDelayedArtifactsMng) newContentWriter() (*content.ContentWriter, error) {
	writer, err := content.NewContentWriter("delayed_artifacts", true, false)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func (mng *TransferDelayedArtifactsMng) writeDelayedArtifactContent(artifact FileRepresentation) error {
	log.Debug(fmt.Sprintf("Deplay the upload of file '%s'. Writing it to be uploaded later...", path.Join(artifact.Repo, artifact.Path, artifact.Name)))
	mng.deployedWriter.writer.Write(artifact)
	mng.deployedWriter.delayedArtifactCount++
	// If file contains maximum number of delayedArtifacts - create and write to a new delayedArtifacts file
	if mng.deployedWriter.delayedArtifactCount == maxDelayedArtifactsInFile {
		err := mng.closeWriter()
		if err != nil {
			return err
		}
		// Initialize variables for new delayedArtifacts file
		mng.deployedWriter.fileIndex++
		mng.deployedWriter.writer, err = mng.newContentWriter()
		if err != nil {
			return err
		}
		mng.deployedWriter.delayedArtifactCount = 0
	}
	return nil
}

func (mng *TransferDelayedArtifactsMng) closeWriter() error {
	// Close content writer and add output file to the array in the manager.
	if mng.deployedWriter.writer == nil {
		return nil
	}
	err := mng.deployedWriter.writer.Close()
	if err != nil {
		return err
	}

	if mng.deployedWriter.writer.GetFilePath() != "" {
		mng.filesToConsume = append(mng.filesToConsume, mng.deployedWriter.writer.GetFilePath())
	}
	return err
}

type DelayedArtifactsFile struct {
	DelayedArtifacts []FileRepresentation `json:"delayed_artifacts,omitempty"`
}

func handleDelayedArtifactsFiles(filesToConsume []string, base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) error {
	log.Info("Starting to handle delayed artifacts uploads...")
	manager := newTransferManager(base, delayUploadComparisonFunctions)
	action := func(optionalPcDetails producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper) error {
		return consumeDelayedArtifactsFiles(filesToConsume, uploadTokensChan, base, delayHelper)
	}
	err := manager.doTransfer(false, action)
	if err == nil {
		log.Info("Done handling delayed artifacts uploads.")
	}
	return err
}

func consumeDelayedArtifactsFiles(filesToConsume []string, uploadTokensChan chan string, base phaseBase, delayHelper delayUploadHelper) error {
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

		err = uploadByChunks(delayedArtifactsFile.DelayedArtifacts, uploadTokensChan, base, delayHelper)
		if err != nil {
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
	shouldDelayFunctions    []shouldDelayUpload
	delayedArtifactsChannel chan FileRepresentation
}

// Decide whether to delay the deployment of a file by running over the shouldDelayUpload array.
// This is an array to allow removing a single comparison when its delay is no longer needed.
func (delayHelper delayUploadHelper) delayUploadIfNecessary(file FileRepresentation) bool {
	for _, shouldDelay := range delayHelper.shouldDelayFunctions {
		if shouldDelay(file.Name) {
			delayHelper.delayedArtifactsChannel <- file
			return true
		}
	}
	return false
}
