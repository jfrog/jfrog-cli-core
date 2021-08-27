package npm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadPackageInfoFromTarball(t *testing.T) {
	npmPublish := NewNpmPublishCommand()
	npmPublish.packedFilePath = filepath.Join("..", "testdata", "npm", "npm-example-0.0.3.tgz")
	err := npmPublish.readPackageInfoFromTarball()
	assert.NoError(t, err)

	assert.Equal(t, "npm-example", npmPublish.packageInfo.Name)
	assert.Equal(t, "0.0.3", npmPublish.packageInfo.Version)
}
