package transferconfig

import (
	"archive/zip"
	"bytes"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"golang.org/x/exp/slices"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchiveConfig(t *testing.T) {
	expectedConfigXml := "<config></config>"
	exportPath := filepath.Join("..", "testdata", "artifactory_export")
	buf, err := archiveConfig(exportPath, expectedConfigXml)
	assert.NoError(t, err)

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	assert.NoError(t, err)

	expectedFiles := append(slices.Clone(neededFiles), "artifactory.config.xml")
	assert.Len(t, zipReader.File, len(expectedFiles))
	for _, zipFile := range zipReader.File {
		assert.Contains(t, expectedFiles, zipFile.Name)
		if zipFile.Name == "artifactory.config.xml" {
			f, err := zipFile.Open()
			assert.NoError(t, err)
			defer func(file io.ReadCloser) {
				assert.NoError(t, file.Close())
			}(f)
			actualConfigXml, err := io.ReadAll(f)
			assert.NoError(t, err)
			assert.Equal(t, expectedConfigXml, string(actualConfigXml))
		}
	}
}

func initHandleTypoInAccessBootstrapTest(t *testing.T) (exportDir string, cleanupFunc func()) {
	exportDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	testDataPath := filepath.Join("..", "testdata", "artifactory_export")
	assert.NoError(t, biutils.CopyDir(testDataPath, exportDir, true, nil))
	cleanupFunc = func() {
		assert.NoError(t, fileutils.RemoveTempDir(exportDir), "Couldn't remove temp dir")
	}
	return
}

func TestHandleTypoInAccessBootstrapNoTypo(t *testing.T) {
	exportDir, cleanupFunc := initHandleTypoInAccessBootstrapTest(t)
	defer cleanupFunc()

	// Test access.bootstrap.json
	assert.NoError(t, handleTypoInAccessBootstrap(exportDir))
	assert.FileExists(t, filepath.Join(exportDir, "etc", "access.bootstrap.json"))
}

func TestHandleTypoInAccessBootstrapWithTypo(t *testing.T) {
	exportDir, cleanupFunc := initHandleTypoInAccessBootstrapTest(t)
	defer cleanupFunc()

	accessBootstrapPath := filepath.Join(exportDir, "etc", "access.bootstrap.json")
	assert.NoError(t, fileutils.MoveFile(accessBootstrapPath, filepath.Join(exportDir, "etc", "access.boostrap.json")))

	assert.NoError(t, handleTypoInAccessBootstrap(exportDir))
	assert.FileExists(t, accessBootstrapPath)
}

func TestHandleTypoInAccessBootstrapNotExist(t *testing.T) {
	exportDir, cleanupFunc := initHandleTypoInAccessBootstrapTest(t)
	defer cleanupFunc()

	assert.NoError(t, os.Remove(filepath.Join(exportDir, "etc", "access.bootstrap.json")))
	assert.Error(t, handleTypoInAccessBootstrap(exportDir))
}
