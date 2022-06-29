package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestGetErrorsFiles(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	retryableErrorsDirPath, err := coreutils.GetJfrogTransferRetryableDir()
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(retryableErrorsDirPath))

	repoKey := "my-repo-local"
	// Create 3 errors files that belong to the repo.
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 0, 0))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 0, 1))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 1, 0))
	// Create 2 errors files that are distractions.
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, "wrong"+repoKey, 0, 0))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey+"wrong", 0, 1))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, "wrong-"+repoKey+"-wrong", 1, 0))

	paths, err := getErrorsFiles("my-repo-local", true)
	assert.NoError(t, err)
	assert.Len(t, paths, 3)
}

func writeEmptyErrorsFile(path, repoKey string, phase, counter int) error {
	fileName := fmt.Sprintf("%s-%d-%s-%d.json", repoKey, phase, strconv.FormatInt(time.Now().Unix(), 10), counter)
	return ioutil.WriteFile(filepath.Join(path, fileName), nil, 0644)
}
