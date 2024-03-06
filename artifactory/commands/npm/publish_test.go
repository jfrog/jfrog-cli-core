package npm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadPackageInfoFromTarball(t *testing.T) {
	npmPublish := NewNpmPublishCommand()
	npmPublish.packedFilesPath = append(npmPublish.packedFilesPath, filepath.Join("..", "testdata", "npm", "npm-example-0.0.3.tgz"))
	npmPublish.packedFilesPath = append(npmPublish.packedFilesPath, filepath.Join("..", "testdata", "npm", "npm-example-0.0.4.tgz"))

	err := npmPublish.readPackageInfoFromTarball(npmPublish.packedFilesPath[0])
	assert.NoError(t, err)
	assert.Equal(t, "npm-example", npmPublish.packageInfo.Name)
	assert.Equal(t, "0.0.3", npmPublish.packageInfo.Version)

	err = npmPublish.readPackageInfoFromTarball(npmPublish.packedFilesPath[1])
	assert.NoError(t, err)
	assert.Equal(t, "npm-example", npmPublish.packageInfo.Name)
	assert.Equal(t, "0.0.4", npmPublish.packageInfo.Version)

}
