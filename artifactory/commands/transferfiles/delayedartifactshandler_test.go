package transferfiles

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

var delayTestRepoKey = "delay-local-repo"

func TestDelayedArtifactsMng(t *testing.T) {
	// Set testing environment
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	delaysDirPath, err := getJfrogTransferRepoDelaysDir(testRepoKey)
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(delaysDirPath))

	artifactsNumber := 50
	// We reduce the maximum number of entities per file to test the creation of multiple delayed artifacts files.
	originalMaxArtifactsInFile := maxDelayedArtifactsInFile
	maxDelayedArtifactsInFile = 20
	defer func() { maxDelayedArtifactsInFile = originalMaxArtifactsInFile }()
	artifactsChannelMng := createdDelayedArtifactsChannelMng()
	transferDelayedArtifactsToFile, err := newTransferDelayedArtifactsManager(&artifactsChannelMng, testRepoKey, state.ConvertTimeToEpochMilliseconds(time.Now()))
	assert.NoError(t, err)
	var writeWaitGroup sync.WaitGroup
	var readWaitGroup sync.WaitGroup

	// "Writing delayed artifacts to files" mechanism returned error
	var delayedArtifactsErr error
	// Start reading from the delayed artifacts channel, and write artifacts into files.
	readWaitGroup.Add(1)
	go func() {
		defer readWaitGroup.Done()
		delayedArtifactsErr = transferDelayedArtifactsToFile.start()
	}()

	// Add artifacts to the common channel.
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < artifactsNumber; i++ {
			artifactsChannelMng.channel <- api.FileRepresentation{Repo: testRepoKey, Path: "path", Name: fmt.Sprintf("name%d", i), Size: int64(i)}
		}
	}()

	writeWaitGroup.Wait()
	artifactsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, delayedArtifactsErr)

	// add not relevant files to confuse
	writeEmptyConfuseFiles(t, repo1Key)
	writeEmptyConfuseFiles(t, delayTestRepoKey+"-0")

	delayFiles, err := getDelayFiles([]string{testRepoKey})
	assert.NoError(t, err)

	expectedNumberOfFiles := int(math.Ceil(float64(artifactsNumber) / float64(maxDelayedArtifactsInFile)))
	validateDelayedArtifactsFiles(t, delayFiles, expectedNumberOfFiles, artifactsNumber)

	delayCount, storage, err := countDelayFilesContent(delayFiles)
	assert.NoError(t, err)
	assert.Equal(t, int64(1225), storage)
	assert.Equal(t, delayCount, artifactsNumber)
}

func writeEmptyConfuseFiles(t *testing.T, repoKey string) {
	delaysDirPath, err := getJfrogTransferRepoDelaysDir(repoKey)
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(delaysDirPath))
	for i := 0; i < 4; i++ {
		writeEmptyFile(t, delaysDirPath, delayTestRepoKey, i)
	}
	writeEmptyFile(t, delaysDirPath, "wrong-"+testRepoKey, 1)
	writeEmptyFile(t, delaysDirPath, testRepoKey+"-wrong", 1)
	writeEmptyFile(t, delaysDirPath, testRepoKey+"-0-0", 0)
	writeEmptyFile(t, delaysDirPath, testRepoKey+"-1", 22)
	writeEmptyFile(t, delaysDirPath, "wrong-"+testRepoKey+"-wrong", 0)

	delayFiles, err := getDelayFiles([]string{repoKey})
	assert.NoError(t, err)
	assert.Len(t, delayFiles, 9)
}

func writeEmptyFile(t *testing.T, delaysDirPath string, repoName string, index int) {
	fullName := fmt.Sprintf("%s-%d.json", getDelaysFilePrefix(repoName, state.ConvertTimeToEpochMilliseconds(time.Now())), index)
	assert.NoError(t, os.WriteFile(filepath.Join(delaysDirPath, fullName), nil, 0644))
}

// Ensure that all 'delayed artifacts files' have been created and that they contain the expected content
func validateDelayedArtifactsFiles(t *testing.T, delayedArtifactsFile []string, filesNum, artifactsNum int) {
	assert.Lenf(t, delayedArtifactsFile, filesNum, "unexpected number of delayed artifacts files.")
	var entitiesNum int
	for i := 0; i < filesNum; i++ {
		entitiesNum += validateDelayedArtifactsFilesContent(t, delayedArtifactsFile[i])
	}
	assert.Equal(t, artifactsNum, entitiesNum)
}

// Check the number of artifacts and their uniqueness by reading the file's content.
func validateDelayedArtifactsFilesContent(t *testing.T, path string) (entitiesNum int) {
	exists, err := fileutils.IsFileExists(path, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("file: %s does not exist", path))

	delayedArtifacts, err := readDelayFile(path)
	assert.NoError(t, err)

	// Verify all artifacts were written with their unique name
	artifactsNamesMap := make(map[string]bool)
	for _, entity := range delayedArtifacts.DelayedArtifacts {
		if artifactsNamesMap[entity.Name] == true {
			assert.Fail(t, fmt.Sprintf("an artifacts with the uniqe name \"%s\" was written more than once", entity.Name))
			return
		}
		artifactsNamesMap[entity.Name] = true
	}
	return len(delayedArtifacts.DelayedArtifacts)
}
