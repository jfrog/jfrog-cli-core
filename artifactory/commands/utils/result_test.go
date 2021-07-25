package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
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

// Check a case that targetRepository is not written in a deployableArtifacts file and needs to be read from the config file.
func TestUnmarshalDeployableArtifacts(t *testing.T) {
	err, cleanUpJfrogHome := config.CreateDefaultJfrogConfig()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()
	// DeployableArtifact file is changed at runtime so a copy needs to be created.
	tempDeployableArtifacts, err := createTempDeployableArtifactFile()
	// delete DeployableArtifacts tempDir
	defer os.Remove(filepath.Dir(tempDeployableArtifacts))
	gradleConfigFile := path.Join(getTestsDataGradlePath(), "config", "gradle.yaml")
	result, err := UnmarshalDeployableArtifacts(tempDeployableArtifacts, gradleConfigFile)
	assert.NoError(t, err)
	for transferDetails := new(clientutils.FileTransferDetails); result.reader.NextRecord(transferDetails) == nil; transferDetails = new(clientutils.FileTransferDetails) {
		assert.True(t, strings.HasPrefix(transferDetails.TargetPath, "http://localhost:8080/artifactory/"))
		assert.True(t, strings.Contains(transferDetails.TargetPath, "gradle-local-repo"))
	}
}

// createTempDeployableArtifactFile copy a deployableArtifacts file from gradle testdata directory to a tempDir
func createTempDeployableArtifactFile() (string, error) {
	testsDataGradlePath := getTestsDataGradlePath()
	summary, err := os.Open(path.Join(testsDataGradlePath, "deployableArtifacts", "artifacts"))
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	tmpDir, err := fileutils.CreateTempDir()
	if err != nil {
		return "", err
	}
	fileutils.CopyFile(tmpDir, summary.Name())
	return filepath.Join(tmpDir, "artifacts"), nil
}

func getTestsDataGradlePath() string {
	return path.Join("..", "testdata", "gradle")
}
