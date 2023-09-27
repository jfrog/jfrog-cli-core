package scan

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDockerScan(t *testing.T) {

	dockerCmd := NewDockerScanCommand()
	currDir, err := os.Getwd()
	assert.NoError(t, err)
	dockerCmd.dockerFilePath = filepath.Join(currDir, "..", "/testdata/docker-scan/.dockerfile")
	tmpDir, err := fileutils.CreateTempDir()
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	assert.NoError(t, err)

	err = os.Chdir(tmpDir)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Chdir(currDir))
	}()
	err = utils.CopyFile(tmpDir, dockerCmd.dockerFilePath)
	assert.NoError(t, err)

	cleanUp, err := dockerCmd.loadDockerfileToMemory()
	defer cleanUp()
	assert.NoError(t, err)

	err = dockerCmd.buildDockerImage()
	assert.NoError(t, err)

	dockerSaveCmd := exec.Command("docker", "save", "audittag", "-o", ".dockerfile")
	err = dockerSaveCmd.Run()
	assert.NoError(t, err)

	mapping, err := dockerCmd.mapDockerLayerToCommand()
	assert.NoError(t, err)

	expected := map[string]services.DockerfileCommandDetails{
		"61bbeda53374249345217a09cd78040f124ef8bc07a1612a1a186d2468f67d54": {
			LayerHash: "sha256:61bbeda53374249345217a09cd78040f124ef8bc07a1612a1a186d2468f67d54",
			Command:   "#(nop) ADD file:3fcf00866c55150f1ea0a5ef7b8473c39275c1fdbf6a ...",
			Line:      []string{"2"},
		},
		"2467098ed029cf49355e601d9f4b07130504b792b565e7552d33c04e216b6e21": {
			LayerHash: "sha256:2467098ed029cf49355e601d9f4b07130504b792b565e7552d33c04e216b6e21",
			Command:   "RUN /bin/sh -c apt-get update && apt-get install -y curl",
			Line:      []string{"2"},
		},
		"afeaacd91b5d98cbfa94cb3ba7e4c8bb03bec3fbb86cd3ab53d0d1a7a8fa5405": {
			LayerHash: "sha256:afeaacd91b5d98cbfa94cb3ba7e4c8bb03bec3fbb86cd3ab53d0d1a7a8fa5405",
			Command:   "RUN /bin/sh -c curl -sL https://deb.nodesource.com/setup_14. ...",
			Line:      []string{"2"},
		},
		"0db3c21e9706d3fa9e3080766ad3074d344afacd8e043d07344657c04d1fa006": {
			LayerHash: "sha256:0db3c21e9706d3fa9e3080766ad3074d344afacd8e043d07344657c04d1fa006",
			Command:   "RUN /bin/sh -c apt-get install -y nodejs",
			Line:      []string{"10"},
		},
	}
	assert.Equal(t, expected, mapping)
}
