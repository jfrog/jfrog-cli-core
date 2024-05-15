package jobsummariesimpl

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildUiUrl(t *testing.T) {
	gh := &GithubSummaryRtUploadImpl{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "myProject",
	}
	expected := "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey=myProject"
	actual := gh.buildUiUrl("myPath")
	assert.Equal(t, expected, actual)

	gh = &GithubSummaryRtUploadImpl{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "",
	}
	expected = "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey="
	actual = gh.buildUiUrl("myPath")
	assert.Equal(t, expected, actual)
}

func TestBuildInfoTable(t *testing.T) {
	gh := &GithubSummaryBpImpl{}
	gh.Builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}
	expected := "\n\n| 🔢 Build Info | 🕒 Timestamp | \n|---------|------------| \n| [buildName / 123](http://myJFrogPlatform/builds/buildName/123) | May 5, 2024 12:47:20 |\n\n\n"
	assert.Equal(t, expected, gh.buildInfoTable())
}

func TestParseBuildTime(t *testing.T) {
	expected := "Jan 2, 2006 15:04:05"
	actual := parseBuildTime("2006-01-02T15:04:05.000-0700")
	assert.Equal(t, expected, actual)

	expected = "N/A"
	actual = parseBuildTime("")
	assert.Equal(t, expected, actual)
}
