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

var generateDiffAqlQueryTestCases = []struct {
	paginationOffset       int
	disabledDistinctiveAql bool
	expectedAql            string
}{
	{0, false, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000)"},
	{0, true, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000).distinct(false)"},
	{2, false, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000)"},
	{2, true, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000).distinct(false)"},
}

func TestGenerateDiffAqlQuery(t *testing.T) {
	for _, testCase := range generateDiffAqlQueryTestCases {
		t.Run("", func(*testing.T) {
			results := generateDiffAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}

var generateDockerManifestAqlQueryTestCases = []struct {
	paginationOffset       int
	disabledDistinctiveAql bool
	expectedAql            string
}{
	{0, false, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000)"},
	{0, true, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000).distinct(false)"},
	{2, false, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000)"},
	{2, true, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000).distinct(false)"},
}

func TestGenerateDockerManifestAqlQuery(t *testing.T) {
	for _, testCase := range generateDockerManifestAqlQueryTestCases {
		t.Run("", func(*testing.T) {
			results := generateDockerManifestAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}
