package commandssummaries

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/stretchr/testify/assert"
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
	expected := "\n\n|  Build Info |  Time Stamp | \n|---------|------------| \n| [buildName 123](http://myJFrogPlatform/builds/buildName/123) | May 5, 2024 , 12:47:20 |\n\n\n"
	assert.Equal(t, expected, gh.buildInfoTable(builds))
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
