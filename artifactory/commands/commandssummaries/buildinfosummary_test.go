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
					Type:       "generic",
					Properties: nil,
					Id:         "python-example",
					Artifacts: []buildinfo.Artifact{
						{
							Name: "jfrog_python_example-1.0-py3-none-any.whl",
							Path: "dist/jfrog_python_example-1.0-py3-none-any.whl",
						},
					},
				},
				{
					Type:       "npm",
					Properties: nil,
					Id:         "npm-example:0.0.3",
					Artifacts: []buildinfo.Artifact{
						{
							Name: "npm-example-0.0.3.tgz",
							Path: "npm-example/-/npm-example-0.0.3.tgz",
						},
					},
				},
			},
		},
	}
	expected := "\n\n # Modules Published \n\n\n ### `python-example` \n\n\n <pre>📦 python-example\n└── <a href=dist/jfrog_python_example-1.0-py3-none-any.whl target=\"_blank\">jfrog_python_example-1.0-py3-none-any.whl</a>\n\n</pre>\n ### `npm-example:0.0.3` \n\n\n <pre>📦 npm-example:0.0.3\n└── <a href=https://ecosysjfrog.jfrog.io/ui/repos/tree/General/robi-npm-local/npm-example/-/npm-example-0.0.3.tgz target=\"_blank\">npm-example-0.0.3.tgz</a>\n\n</pre>"
	assert.Equal(t, expected, gh.buildInfoModules(builds))
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
