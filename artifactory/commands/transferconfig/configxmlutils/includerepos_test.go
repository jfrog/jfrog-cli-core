package configxmlutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testCases = []struct {
	expectedXml          string
	includedRepositories []string
}{
	{"filter-all", []string{}},
	{"select-local", []string{"default-libs-release-local"}},
	{"select-remote", []string{"default-maven-remote"}},
	{"select-one-from-all", []string{"default-libs-release-local", "default-maven-remote", "default-libs-release"}},
	{"select-all", []string{"default-libs-release-local", "default-libs-snapshot-local", "default-maven-remote",
		"ecosys-generic-local", "default-libs-release", "example-repo-local", "ecosys-npm-remote", "default-libs-snapshot"}},
}

func TestFilterNonIncludedRepositories(t *testing.T) {
	for _, testCase := range testCases {
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

			// Run FilterNonIncludedRepositories and compare
			result, err := FilterNonIncludedRepositories(string(inputConfigXml), testCase.includedRepositories)
			assert.NoError(t, err)
			assert.Equal(t, string(expectedConfigXml), result)
		})
	}
}
