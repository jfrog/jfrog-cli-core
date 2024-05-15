package jobsummariesimpl

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
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

func TestUploadFilesMarkdown(t *testing.T) {
	ga := &GithubSummaryRtUploadImpl{
		uploadTree:        utils.NewFileTree(),
		uploadedArtifacts: ResultsWrapper{},
		PlatformUrl:       "https://myplatform.com/",
		JfrogProjectKey:   "myProject",
	}

	content := []byte(`{
    "results": [
        {
            "sourcePath": "testdata/a/b/c.in",
            "targetPath": "project-testRepo/c.in",
            "rtUrl": "https://platform.jfrog.io/artifactory/"
        },
        {
            "sourcePath": "testdata/a/b/b.in",
            "targetPath": "project-testRepo/b.in",
            "rtUrl": "https://platform.jfrog.io/artifactory/"
        },
        {
            "sourcePath": "testdata/a/b/a.in",
            "targetPath": "project-testRepo/a.in",
            "rtUrl": "https://platform.jfrog.io/artifactory/"
        }
    ]
}`)

	markdown, err := ga.renderContentToMarkdown(content)
	assert.Nil(t, err)
	expected := `
<pre>
📦 project-testRepo
├── 📄 <a href=https://myplatform.com/ui/repos/tree/General/project-testRepo/a.in/?projectKey=myProject target="_blank">a.in</a>
├── 📄 <a href=https://myplatform.com/ui/repos/tree/General/project-testRepo/b.in/?projectKey=myProject target="_blank">b.in</a>
└── 📄 <a href=https://myplatform.com/ui/repos/tree/General/project-testRepo/c.in/?projectKey=myProject target="_blank">c.in</a>
</pre>

`
	assert.Equal(t, expected, markdown)
}
