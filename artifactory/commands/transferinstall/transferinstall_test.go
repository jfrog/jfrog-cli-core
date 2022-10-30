package transferinstall

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDownloadTransferInstall(t *testing.T) {
	testDstPath, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	cmd := NewInstallTransferCommand(nil)
	// make sure base url is set
	assert.Equal(t, dataTransferUrl, cmd.baseDownloadUrl)
	// download latest and make sure exists at the end
	assert.NoError(t, DownloadFiles(dataTransferUrl+"/"+latest, testDstPath, transferPluginFiles))
	for _, file := range transferPluginFiles {
		assert.FileExists(t, file.toPath(testDstPath))
	}
}
