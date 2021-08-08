package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// Checks a case that targetRepository is not written in a deployableArtifacts file and needs to be read from the config file.
func TestUnmarshalDeployableArtifacts(t *testing.T) {
	err, cleanUpJfrogHome := tests.ConfigTestServer(t)
	assert.NoError(t, err)
	defer cleanUpJfrogHome()
	// DeployableArtifact file is changed at runtime so a copy needs to be created.
	tempDeployableArtifacts, err := createTempDeployableArtifactFile()
	// Delete DeployableArtifacts tempDir
	defer os.Remove(filepath.Dir(tempDeployableArtifacts))
	gradleConfigFile := path.Join(getTestsDataGradlePath(), "config", "gradle.yaml")
	result, err := UnmarshalDeployableArtifacts(tempDeployableArtifacts, gradleConfigFile)
	assert.NoError(t, err)
	for transferDetails := new(clientutils.FileTransferDetails); result.reader.NextRecord(transferDetails) == nil; transferDetails = new(clientutils.FileTransferDetails) {
		assert.True(t, strings.HasPrefix(transferDetails.TargetPath, "http://localhost:8080/artifactory/"))
		assert.True(t, strings.Contains(transferDetails.TargetPath, "gradle-local-repo"))
	}
}

// createTempDeployableArtifactFile copies a deployableArtifacts file from gradle testdata directory to a tempDir
func createTempDeployableArtifactFile() (filePath string, err error) {
	filePath = ""
	testsDataGradlePath := getTestsDataGradlePath()
	summary, err := os.Open(path.Join(testsDataGradlePath, "deployableArtifacts", "artifacts"))
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	defer func() {
		e := summary.Close()
		if err == nil {
			err = e
		}
	}()
	tmpDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	fileutils.CopyFile(tmpDir, summary.Name())
	filePath = filepath.Join(tmpDir, "artifacts")
	return
}

func getTestsDataGradlePath() string {
	return path.Join("..", "testdata", "gradle")
}
