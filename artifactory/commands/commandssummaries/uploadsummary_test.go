package commandssummaries

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildUiUrl(t *testing.T) {
	gh := &UploadSummary{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "myProject",
	}
	expected := "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey=myProject"
	actual := gh.buildUiUrl("myPath")
	assert.Equal(t, expected, actual)

	gh = &UploadSummary{
		PlatformUrl:     "https://myplatform.com/",
		JfrogProjectKey: "",
	}
	expected = "https://myplatform.com/ui/repos/tree/General/myPath/?projectKey="
	actual = gh.buildUiUrl("myPath")
	assert.Equal(t, expected, actual)
}
