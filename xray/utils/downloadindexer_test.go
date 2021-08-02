package utils

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestDeleteOldIndexers(t *testing.T) {
	testsDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(testsDir))
	}()
	indexersDir := path.Join(testsDir, "xray-indexer")
	indexersDirsPaths := []string{
		path.Join(indexersDir, "1.0.0"),
		path.Join(indexersDir, "1.2.0"),
		path.Join(indexersDir, "1.3.x-SNAPSHOT"),
	}

	// Test no indexers directory at all
	assert.NoError(t, deleteOldIndexers(indexersDir))

	// Test there are two directories in the indexers directory - nothing should be deleted
	createDummyIndexer(t, indexersDirsPaths[0])
	createDummyIndexer(t, indexersDirsPaths[1])
	assert.NoError(t, deleteOldIndexers(indexersDir))
	assert.True(t, checkIndexerExists(t, indexersDirsPaths[0]))
	assert.True(t, checkIndexerExists(t, indexersDirsPaths[1]))

	// Test there are three directories in the indexers directory - the oldest one (by version) should be deleted
	createDummyIndexer(t, indexersDirsPaths[2])
	assert.NoError(t, deleteOldIndexers(indexersDir))
	assert.False(t, checkIndexerExists(t, indexersDirsPaths[0]))
	assert.True(t, checkIndexerExists(t, indexersDirsPaths[1]))
	assert.True(t, checkIndexerExists(t, indexersDirsPaths[2]))
}

func createDummyIndexer(t *testing.T, dirPath string) {
	err := os.MkdirAll(dirPath, 0777)
	assert.NoError(t, err)
	fullPath := path.Join(dirPath, indexerFileName)
	file, err := os.Create(fullPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()
	_, err = file.Write([]byte(fullPath))
	assert.NoError(t, err)
}

func checkIndexerExists(t *testing.T, dirPath string) bool {
	indexerPath := path.Join(dirPath, indexerFileName)
	exists, err := fileutils.IsFileExists(indexerPath, true)
	assert.NoError(t, err)
	return exists
}
