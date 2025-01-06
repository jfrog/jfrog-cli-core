package python

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePipConfigManually(t *testing.T) {
	// Define the test parameters
	customConfigPath := filepath.Join(t.TempDir(), "/tmp/test/pip.conf")
	repoWithCredsUrl := "https://example.com/simple/"
	expectedContent := "[global]\nindex-url = https://example.com/simple/\n"

	// Call the function under test
	err := CreatePipConfigManually(customConfigPath, repoWithCredsUrl)

	// Assert no error occurred
	assert.NoError(t, err)

	// Verify the file exists and has the correct content
	fileContent, err := os.ReadFile(customConfigPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(fileContent))
}
