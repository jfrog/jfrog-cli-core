package configxmlutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/stretchr/testify/assert"
)

var testCases = []struct {
	expectedXml          string
	includedRepositories []string
	excludedRepositories []string
}{
	{"filter-all", []string{}, []string{"*"}},
	{"select-local", []string{"default-libs-release-local"}, []string{}},
	{"select-remote", []string{"default-maven-remote"}, []string{}},
	{"select-release-bundle", []string{"release-bundles"}, []string{}},
	{"select-one-from-all", []string{"default-libs-release-local", "default-maven-remote", "default-libs-release", "release-bundles"}, []string{}},
	{"select-all", []string{"default-libs-release-local", "default-libs-snapshot-local", "default-maven-remote", "artifactory-build-info", "ecosys-build-info",
		"ecosys-generic-local", "default-libs-release", "example-repo-local", "ecosys-npm-remote", "default-go-remote", "default-libs-snapshot", "release-bundles"}, []string{}},
}

func TestRemoveNonIncludedRepositories(t *testing.T) {
	for _, testCase := range testCases {
		includeExcludeFilter := &utils.IncludeExcludeFilter{
			IncludePatterns: testCase.includedRepositories,
			ExcludePatterns: testCase.excludedRepositories,
		}
		t.Run(testCase.expectedXml, func(t *testing.T) {
			testCasesDir := filepath.Join("..", "..", "testdata", "config_xmls_exclude_repos")

			// Read input artifactory.config.xml
			inputConfigXmlPath := filepath.Join(testCasesDir, "input", "artifactory.config.xml")
			inputConfigXml, err := os.ReadFile(inputConfigXmlPath)
			assert.NoError(t, err)

			// Read expected artifactory.config.xml
			expectedConfigXmlPath := filepath.Join(testCasesDir, "cases", testCase.expectedXml+".xml")
			expectedConfigXml, err := os.ReadFile(expectedConfigXmlPath)
			assert.NoError(t, err)

			// Run RemoveNonIncludedRepositories and compare
			result, err := RemoveNonIncludedRepositories(string(inputConfigXml), includeExcludeFilter)
			assert.NoError(t, err)
			assert.Equal(t, string(expectedConfigXml), result)
		})
	}
}
