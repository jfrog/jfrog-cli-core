package npm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadPackageInfoFromTarball(t *testing.T) {
	npmPublish := NewNpmPublishCommand()
	npmPublish.packedFilePaths = append(npmPublish.packedFilePaths, filepath.Join("..", "testdata", "npm", "npm-example-0.0.3.tgz"))
	npmPublish.packedFilePaths = append(npmPublish.packedFilePaths, filepath.Join("..", "testdata", "npm", "npm-example-0.0.4.tgz"))

	err := npmPublish.readPackageInfoFromTarball(npmPublish.packedFilePaths[0])
	assert.NoError(t, err)
	assert.Equal(t, "npm-example", npmPublish.packageInfo.Name)
	assert.Equal(t, "0.0.3", npmPublish.packageInfo.Version)

	err = npmPublish.readPackageInfoFromTarball(npmPublish.packedFilePaths[1])
	assert.NoError(t, err)
	assert.Equal(t, "npm-example", npmPublish.packageInfo.Name)
	assert.Equal(t, "0.0.4", npmPublish.packageInfo.Version)

}
