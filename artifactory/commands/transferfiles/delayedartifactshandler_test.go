package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"math"
	"sync"
	"testing"
)

func TestDelayedArtifactsMng(t *testing.T) {
	// Set testing environment
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	artifactsNumber := 40
	// We reduce the maximum number of entities per file to test the creation of multiple delayed artifacts files.
	maxDelayedArtifactsInFile = 20
	artifactsChannelMng := createdDelayedArtifactsChannelMng()
	transferDelayedArtifactsToFile := newTransferDelayedArtifactsToFile(&artifactsChannelMng)

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
			artifactsChannelMng.channel <- FileRepresentation{Repo: testRepoKey, Path: "path", Name: fmt.Sprintf("name%d", i)}
		}
	}()

	writeWaitGroup.Wait()
	artifactsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, delayedArtifactsErr)
	expectedNumberOfFiles := int(math.Ceil(float64(artifactsNumber) / float64(maxDelayedArtifactsInFile)))
	validateDelayedArtifactsFiles(t, transferDelayedArtifactsToFile.filesToConsume, expectedNumberOfFiles, maxDelayedArtifactsInFile)
}

// Ensure that all 'delayed artifacts files' have been created and that they contain the expected content
func validateDelayedArtifactsFiles(t *testing.T, delayedArtifactsFile []string, filesNum, artifactsNum int) {
	assert.Equal(t, filesNum, len(delayedArtifactsFile), "unexpected number of delayed artifacts files.")
	for i := 0; i < filesNum; i++ {
		entitiesNum := validateDelayedArtifactsFilesContent(t, delayedArtifactsFile[i])
		assert.Equal(t, artifactsNum, entitiesNum)
	}
}

// Check the number of artifacts and their uniqueness by reading the file's content.
func validateDelayedArtifactsFilesContent(t *testing.T, path string) (entitiesNum int) {
	exists, err := fileutils.IsFileExists(path, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("file: %s does not exist", path))

	var content []byte
	content, err = fileutils.ReadFile(path)
	assert.NoError(t, err)

	delayedArtifacts := new(DelayedArtifactsFile)
	assert.NoError(t, json.Unmarshal(content, &delayedArtifacts))

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
