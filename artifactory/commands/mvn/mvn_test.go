package mvn

import (
	"encoding/json"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdateBuildInfoArtifactsWithTargetRepo(t *testing.T) {
	vConfig := viper.New()
	vConfig.Set(build.DeployerPrefix+build.SnapshotRepo, "snapshots")
	vConfig.Set(build.DeployerPrefix+build.ReleaseRepo, "releases")

	tempDir := t.TempDir()
	assert.NoError(t, io.CopyDir(filepath.Join("testdata", "buildinfo_files"), tempDir, true, nil))

	buildName := "buildName"
	buildNumber := "1"
	mc := MvnCommand{
		configuration: build.NewBuildConfiguration(buildName, buildNumber, "", ""),
	}

	buildInfoFilePath := filepath.Join(tempDir, "buildinfo1")

	err := mc.updateBuildInfoArtifactsWithDeploymentRepo(vConfig, buildInfoFilePath)
	assert.NoError(t, err)

	buildInfoContent, err := os.ReadFile(buildInfoFilePath)
	assert.NoError(t, err)

	var buildInfo entities.BuildInfo
	assert.NoError(t, json.Unmarshal(buildInfoContent, &buildInfo))

	assert.Len(t, buildInfo.Modules, 2)
	modules := buildInfo.Modules

	firstModule := modules[0]
	assert.Len(t, firstModule.Artifacts, 0)
	excludedArtifacts := firstModule.ExcludedArtifacts
	assert.Len(t, excludedArtifacts, 2)
	assert.Equal(t, "snapshots", excludedArtifacts[0].OriginalDeploymentRepo)
	assert.Equal(t, "snapshots", excludedArtifacts[1].OriginalDeploymentRepo)

	secondModule := modules[1]
	assert.Len(t, secondModule.ExcludedArtifacts, 0)
	artifacts := secondModule.Artifacts
	assert.Len(t, artifacts, 2)
	assert.Equal(t, "releases", artifacts[0].OriginalDeploymentRepo)
	assert.Equal(t, "releases", artifacts[1].OriginalDeploymentRepo)
}
