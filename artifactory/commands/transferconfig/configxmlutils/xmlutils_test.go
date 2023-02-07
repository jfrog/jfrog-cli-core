package configxmlutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveAllRepositories(t *testing.T) {
	testCasesDir := filepath.Join("..", "..", "testdata", "config_xmls_exclude_repos")

	// Read input artifactory.config.xml
	inputConfigXmlPath := filepath.Join(testCasesDir, "input", "artifactory.config.xml")
	inputConfigXml, err := os.ReadFile(inputConfigXmlPath)
	assert.NoError(t, err)

	// Read expected artifactory.config.xml
	expectedConfigXmlPath := filepath.Join(testCasesDir, "cases", "filter-all.xml")
	expectedConfigXml, err := os.ReadFile(expectedConfigXmlPath)
	assert.NoError(t, err)

	// Remove all repositories and compare
	actualConfigXml, err := RemoveAllRepositories(string(inputConfigXml))
	assert.NoError(t, err)
	assert.Equal(t, string(expectedConfigXml), actualConfigXml)
}
