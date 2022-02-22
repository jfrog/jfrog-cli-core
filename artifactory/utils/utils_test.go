package utils

import (
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestGetHomeDir(t *testing.T) {
	homeDir, err := coreutils.GetJfrogHomeDir()
	assert.NoError(t, err)
	secPath, err := coreutils.GetJfrogSecurityDir()
	assert.NoError(t, err)
	secFile, err := coreutils.GetJfrogSecurityConfFilePath()
	assert.NoError(t, err)
	certsPath, err := coreutils.GetJfrogCertsDir()
	assert.NoError(t, err)

	assert.Equal(t, secPath, filepath.Join(homeDir, coreutils.JfrogSecurityDirName))
	assert.Equal(t, secFile, filepath.Join(secPath, coreutils.JfrogSecurityConfFile))
	assert.Equal(t, certsPath, filepath.Join(secPath, coreutils.JfrogCertsDirName))
}
