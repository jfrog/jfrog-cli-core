package transferconfig

import (
	"archive/zip"
	"bytes"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchiveConfig(t *testing.T) {
	expectedConfigXml := "<config></config>"
	exportPath := filepath.Join("..", "testdata", "artifactory_export", "regular")
	buf, err := archiveConfig(exportPath, expectedConfigXml)
	assert.NoError(t, err)

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	assert.NoError(t, err)

	expectedFiles := append(neededFiles, "artifactory.config.xml")
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

// In some versions of Artifactory, the file access.bootstrap.json has a typo in its name: access.boostrap.json.
// This tests this scenario.
func TestArchiveConfigWithTypoInAccessBootstrap(t *testing.T) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer assert.NoError(t, fileutils.RemoveTempDir(tmpDir), "Couldn't remove temp dir")
	testDataPath := filepath.Join("..", "testdata", "artifactory_export", "access_bootstrap_typo")
	defer assert.NoError(t, fileutils.CopyDir(testDataPath, tmpDir, true, nil))

	expectedConfigXml := "<config></config>"
	_, err = archiveConfig(tmpDir, expectedConfigXml)
	assert.NoError(t, err)
}
