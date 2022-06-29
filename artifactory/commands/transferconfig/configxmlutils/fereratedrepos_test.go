package configxmlutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceUrlInFederatedrepos(t *testing.T) {
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

			result, err := ReplaceUrlsInFederatedrepos(string(inputConfigXml), "http://localhost:8081/artifactory", "https://acme.jfrog.io/artifactory")
			assert.NoError(t, err)
			assert.Equal(t, string(expectedConfigXml), result)
		})
	}
}
