package utils

import (
	biutils "github.com/jfrog/build-info-go/utils"
	ioutils "github.com/jfrog/gofrog/io"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

// Checks a case that targetRepository is not written in a deployableArtifacts file and needs to be read from the config file.
func TestUnmarshalDeployableArtifacts(t *testing.T) {
	cleanUpJfrogHome, err := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUpJfrogHome()
	// DeployableArtifact file is changed at runtime so a copy needs to be created.
	tempDeployableArtifacts, err := createTempDeployableArtifactFile()
	assert.NoError(t, err)
	// Delete DeployableArtifacts tempDir
	defer testsutils.RemoveAllAndAssert(t, filepath.Dir(tempDeployableArtifacts))
	gradleConfigFile := path.Join(getTestsDataGradlePath(), "config", "gradle.yaml")
	result, err := UnmarshalDeployableArtifacts(tempDeployableArtifacts, gradleConfigFile, false)
	assert.NoError(t, err)
	for transferDetails := new(clientutils.FileTransferDetails); result.reader.NextRecord(transferDetails) == nil; transferDetails = new(clientutils.FileTransferDetails) {
		assert.Equal(t, transferDetails.RtUrl, "http://localhost:8080/artifactory/")
		assert.True(t, strings.HasPrefix(transferDetails.TargetPath, "gradle-local-repo"))
	}
}

// createTempDeployableArtifactFile copies a deployableArtifacts file from gradle testdata directory to a tempDir
func createTempDeployableArtifactFile() (filePath string, err error) {
	filePath = ""
	testsDataGradlePath := getTestsDataGradlePath()
	summary, err := os.Open(path.Join(testsDataGradlePath, "deployableArtifacts", "artifacts"))
	if errorutils.CheckError(err) != nil {
		return
	}
	defer ioutils.Close(summary, &err)
	tmpDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	err = biutils.CopyFile(tmpDir, summary.Name())
	if err != nil {
		return
	}
	filePath = filepath.Join(tmpDir, "artifacts")
	return
}

func getTestsDataGradlePath() string {
	return path.Join("..", "testdata", "gradle")
}
