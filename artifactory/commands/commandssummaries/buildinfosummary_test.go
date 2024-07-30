package commandssummaries

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInfoTable(t *testing.T) {
	gh := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}
	assert.Equal(t, getTestDataFile(t, "table.md"), gh.buildInfoTable(builds))
}

func TestBuildInfoModules(t *testing.T) {
	gh := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
			Modules: []buildinfo.Module{
				{
					Type: buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact1",
						Path:                   "path/to/artifact1",
						OriginalDeploymentRepo: "libs-release",
					},
						{
							Name:                   "artifact2",
							Path:                   "path/to/artifact2",
							OriginalDeploymentRepo: "libs-snapshot",
						}},
					// Validate that dependencies don't show.
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					},
					},
				},
				{
					// Validate that ignored types don't show.
					Type: buildinfo.Gradle,
					Artifacts: []buildinfo.Artifact{
						{
							Name:                   "gradleArtifact",
							Path:                   "dir/gradleArtifact",
							OriginalDeploymentRepo: "gradle-local",
						},
					},
				},
			},
		},
	}

	assert.Equal(t, getTestDataFile(t, "modules.md"), gh.buildInfoModules(builds))
}

func getTestDataFile(t *testing.T, fileName string) string {
	modulesPath := filepath.Join(".", "testdata", fileName)
	content, err := os.ReadFile(modulesPath)
	assert.NoError(t, err)
	contentStr := string(content)
	if coreutils.IsWindows() {
		log.Info(strings.Contains(contentStr, "\r\n"))
		contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	}
	return contentStr
}

func TestParseBuildTime(t *testing.T) {
	// Test format
	actual := parseBuildTime("2006-01-02T15:04:05.000-0700")
	expected := "Jan 2, 2006 , 15:04:05"
	assert.Equal(t, expected, actual)
	// Test invalid format
	expected = "N/A"
	actual = parseBuildTime("")
	assert.Equal(t, expected, actual)
}
