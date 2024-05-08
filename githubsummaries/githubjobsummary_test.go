package githubsummaries

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/githubsummariesimpl"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildUiUrl(t *testing.T) {
	gh := &githubsummariesimpl.GithubSummaryRtUploadImpl{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "myProject",
	}
	expected := "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey=myProject"
	actual := gh.BuildUiUrl("myPath")
	assert.Equal(t, expected, actual)

	gh = &githubsummariesimpl.GithubSummaryRtUploadImpl{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "",
	}
	expected = "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey="
	actual = gh.BuildUiUrl("myPath")
	assert.Equal(t, expected, actual)
}

func TestBuildInfoTable(t *testing.T) {
	gh := &githubsummariesimpl.GithubSummaryBpImpl{}
	gh.Builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}
	expected := "| 🔢 Build Info | 🕒 Timestamp | \n|---------|------------| \n| [buildName / 123](http://myJFrogPlatform/builds/buildName/123) | May 5, 2024 12:47:20 |\n"
	assert.Equal(t, expected, gh.BuildInfoTable())
}

func TestParseBuildTime(t *testing.T) {
	expected := "Jan 2, 2006 15:04:05"
	actual := githubsummariesimpl.ParseBuildTime("2006-01-02T15:04:05.000-0700")
	assert.Equal(t, expected, actual)

	expected = "N/A"
	actual = githubsummariesimpl.ParseBuildTime("")
	assert.Equal(t, expected, actual)
}
