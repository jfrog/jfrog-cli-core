package buildinfo

import (
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
)

func TestPrintBuildInfoLink(t *testing.T) {
	timeNow := time.Now()
	buildTime := strconv.FormatInt(timeNow.UnixNano()/1000000, 10)
	var linkTypes = []struct {
		majorVersion  int
		buildTime     time.Time
		buildInfoConf *build.BuildConfiguration
		serverDetails config.ServerDetails
		expected      string
	}{
		// Test platform URL
		{5, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{Url: "http://localhost:8081/"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{6, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{Url: "http://localhost:8081/"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{7, timeNow, build.NewBuildConfiguration("test", "1", "6", ""),
			config.ServerDetails{Url: "http://localhost:8082/"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=artifactory-build-info"},
		{7, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{Url: "http://localhost:8082/"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=cli-build-info&projectKey=cli"},

		// Test Artifactory URL
		{5, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{ArtifactoryUrl: "http://localhost:8081/artifactory"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{6, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{ArtifactoryUrl: "http://localhost:8081/artifactory/"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{7, timeNow, build.NewBuildConfiguration("test", "1", "6", ""),
			config.ServerDetails{ArtifactoryUrl: "http://localhost:8082/artifactory"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=artifactory-build-info"},
		{7, timeNow, build.NewBuildConfiguration("test", "1", "6", "cli"),
			config.ServerDetails{ArtifactoryUrl: "http://localhost:8082/artifactory/"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=cli-build-info&projectKey=cli"},
	}

	for i := range linkTypes {
		buildPubConf := &BuildPublishCommand{
			linkTypes[i].buildInfoConf,
			&linkTypes[i].serverDetails,
			nil,
			true,
			nil,
		}
		buildPubComService, err := buildPubConf.getBuildInfoUiUrl(linkTypes[i].majorVersion, linkTypes[i].buildTime)
		assert.NoError(t, err)
		assert.Equal(t, buildPubComService, linkTypes[i].expected)
	}
}
