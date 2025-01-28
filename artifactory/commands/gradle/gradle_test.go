package gradle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateInitScript(t *testing.T) {
	config := InitScriptAuthConfig{
		ArtifactoryURL:           "http://example.com/artifactory",
		ArtifactoryRepositoryKey: "example-repo",
		ArtifactoryUsername:      "user",
		ArtifactoryAccessToken:   "token",
	}
	script, err := GenerateInitScript(config)
	assert.NoError(t, err)
	assert.Contains(t, script, "http://example.com/artifactory")
	assert.Contains(t, script, "example-repo")
	assert.Contains(t, script, "user")
	assert.Contains(t, script, "token")
}

func TestWriteInitScript(t *testing.T) {
	// Set up a temporary directory for testing
	tempDir := t.TempDir()
	t.Setenv(UserHomeEnv, tempDir)

	initScript := "test init script content"

	err := WriteInitScript(initScript)
	assert.NoError(t, err)

	// Verify the init script was written to the correct location
	expectedPath := filepath.Join(tempDir, "init.d", InitScriptName)
	content, err := os.ReadFile(expectedPath)
	assert.NoError(t, err)
	assert.Equal(t, initScript, string(content))
}
