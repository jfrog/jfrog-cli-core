package transferfiles

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

var convertResultsToFileRepresentationTestCases = []struct {
	input          servicesUtils.ResultItem
	expectedOutput api.FileRepresentation
}{
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: "path-in-repo", Name: "file-name", Type: "file", Size: 100},
		api.FileRepresentation{Repo: repo1Key, Path: "path-in-repo", Name: "file-name", Size: 100},
	},
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: "path-in-repo", Name: "folder-name", Type: "folder"},
		api.FileRepresentation{Repo: repo1Key, Path: "path-in-repo/folder-name"},
	},
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: ".", Name: "folder-name", Type: "folder"},
		api.FileRepresentation{Repo: repo1Key, Path: "folder-name"},
	},
}

func TestConvertResultsToFileRepresentation(t *testing.T) {
	for _, testCase := range convertResultsToFileRepresentationTestCases {
		files := convertResultsToFileRepresentation([]servicesUtils.ResultItem{testCase.input})
		assert.Equal(t, []api.FileRepresentation{testCase.expectedOutput}, files)
	}
}
