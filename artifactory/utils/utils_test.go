package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"
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

// Unmarshal filePath's content into a generic struct.
// filePath - Path to json file.
// loadTarget -  Parsing filePath's content into 'loadTarget'
func unmarshal(filePath string, loadTarget interface{}) (err error) {
	var jsonFile *os.File
	jsonFile, err = os.Open(filePath)
	if err != nil {
		return
	}
	defer func() {
		closeErr := jsonFile.Close()
		if err == nil {
			err = closeErr
		}
	}()
	var byteValue []byte
	byteValue, err = ioutil.ReadAll(jsonFile)
	if err != nil {
		return
	}
	return json.Unmarshal(byteValue, &loadTarget)
}
