package configxmlutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveFederatedMembers(t *testing.T) {
	testCasesDir := filepath.Join("..", "..", "testdata", "config_xmls_federated_repos")
	files, err := os.ReadDir(filepath.Join(testCasesDir, "input"))
	assert.NoError(t, err)

	for i := 0; i < len(files); i++ {
		testId := fmt.Sprint(i)
		t.Run(testId, func(t *testing.T) {
			inputConfigXmlPath := filepath.Join(testCasesDir, "input", "artifactory.config-"+testId+".xml")
			inputConfigXml, err := os.ReadFile(inputConfigXmlPath)
			assert.NoError(t, err)

			expectedConfigXmlPath := filepath.Join(testCasesDir, "expected", "artifactory.config-"+testId+".xml")
			expectedConfigXml, err := os.ReadFile(expectedConfigXmlPath)
			assert.NoError(t, err)

			result, federatedMembersRemoved, err := RemoveFederatedMembers(string(inputConfigXml))
			assert.NoError(t, err)
			assert.Equal(t, federatedMembersRemoved, strings.Contains(string(inputConfigXml), "federatedMembers"))
			assert.Equal(t, string(expectedConfigXml), result)
		})
	}
}
