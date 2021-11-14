package buildinfo

import (
	artifactoryUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestPrintBuildInfoLink(t *testing.T) {
	buildTime := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	var linkTypes = []struct {
		majorVersion  int
		buildTime     time.Time
		buildInfoConf artifactoryUtils.BuildConfiguration
		serverDetails config.ServerDetails
		expected      string
	}{
		{5, time.Now(), artifactoryUtils.BuildConfiguration{BuildName: "test", BuildNumber: "1", Module: "6", Project: "cli"},
			config.ServerDetails{Url: "http://localhost:8081/"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{6, time.Now(), artifactoryUtils.BuildConfiguration{BuildName: "test", BuildNumber: "1", Module: "6", Project: "cli"},
			config.ServerDetails{Url: "http://localhost:8081/"}, "http://localhost:8081/artifactory/webapp/#/builds/test/1"},
		{7, time.Now(), artifactoryUtils.BuildConfiguration{BuildName: "test", BuildNumber: "1", Module: "6", Project: ""},
			config.ServerDetails{Url: "http://localhost:8082/"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=artifactory-build-info"},
		{7, time.Now(), artifactoryUtils.BuildConfiguration{BuildName: "test", BuildNumber: "1", Module: "6", Project: "cli"},
			config.ServerDetails{Url: "http://localhost:8082/"}, "http://localhost:8082/ui/builds/test/1/" + buildTime + "/published?buildRepo=cli-build-info&projectKey=cli"},
	}

	for _, linkType := range linkTypes {
		buildPubConf := &BuildPublishCommand{
			&linkType.buildInfoConf,
			&linkType.serverDetails,
			nil,
			true,
			nil,
		}
		buildPubComService := buildPubConf.getBuildInfoUiUrl(linkType.majorVersion, linkType.buildTime)
		assert.Equal(t, buildPubComService, linkType.expected)
	}
}
