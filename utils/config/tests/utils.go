package tests

import (
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

const certsConversionResources = "testdata/config/configconversion"
const encryptionResources = "testdata/config/encryption"

// Set JFROG_CLI_HOME_DIR environment variable to be a new temp directory
func CreateTempEnv(t *testing.T, copyEncryptionKey bool) (cleanUp func()) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	assert.NoError(t, err)
	oldHome := os.Getenv(coreutils.HomeDir)
	testsutils.SetEnvAndAssert(t, coreutils.HomeDir, tmpDir)
	copyResources(t, certsConversionResources, tmpDir)
	if copyEncryptionKey {
		copyResources(t, encryptionResources, tmpDir)
	}
	return func() {
		testsutils.RemoveAllAndAssert(t, tmpDir)
		testsutils.SetEnvAndAssert(t, coreutils.HomeDir, oldHome)
	}
}

func copyResources(t *testing.T, sourcePath string, destPath string) {
	assert.NoError(t, biutils.CopyDir(sourcePath, destPath, true, nil))
}
