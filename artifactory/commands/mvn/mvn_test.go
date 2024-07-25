package mvn

import (
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdateBuildInfoArtifactsWithTargetRepo(t *testing.T) {
	vConfig := viper.New()
	vConfig.Set("deployer.snapshotrepo", "snapshots")
	vConfig.Set("deployer.releaserepo", "releases")

	tempDir := t.TempDir()
	assert.NoError(t, io.CopyDir(filepath.Join("testdata", "build_info_without_targetrepo"), tempDir, true, nil))

	buildInfoService := build.CreateBuildInfoService()
	buildInfoService.SetTempDirPath(tempDir)
	buildName := "buildName"
	buildNumber := "1"
	mc := MvnCommand{
		configuration: build.NewBuildConfiguration(buildName, buildNumber, "", ""),
	}

	err := mc.updateBuildInfoArtifactsWithTargetRepo(vConfig, buildInfoService)
	assert.NoError(t, err)

	mvnBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, "")
	assert.NoError(t, err)
	buildInfo, err := mvnBuild.ToBuildInfo()
	assert.NoError(t, err)

	assert.Len(t, buildInfo.Modules, 2)
	modules := buildInfo.Modules

	firstModule := modules[0]
	assert.Len(t, firstModule.Artifacts, 0)
	excludedArtifacts := firstModule.ExcludedArtifacts
	assert.Len(t, excludedArtifacts, 2)
	log.Info(excludedArtifacts)
	assert.Equal(t, "snapshots", excludedArtifacts[0].OriginalRepo)
	assert.Equal(t, "snapshots", excludedArtifacts[1].OriginalRepo)

	secondModule := modules[1]
	assert.Len(t, secondModule.ExcludedArtifacts, 0)
	artifacts := secondModule.Artifacts
	log.Info(artifacts)
	assert.Len(t, artifacts, 2)
	assert.Equal(t, "releases", artifacts[0].OriginalRepo)
	assert.Equal(t, "releases", artifacts[1].OriginalRepo)
}
