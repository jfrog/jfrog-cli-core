package gradle

import (
	"github.com/stretchr/testify/require"
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

func TestWriteInitScriptWithBackup(t *testing.T) {
	tests := []struct {
		name           string
		existingScript bool
		expectedBackup bool
	}{
		{name: "No existing init.gradle", existingScript: false, expectedBackup: false},
		{name: "Existing init.gradle", existingScript: true, expectedBackup: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up a temporary directory to act as the Gradle user home
			tempDir := t.TempDir()
			require.NoError(t, os.Setenv("GRADLE_USER_HOME", tempDir))
			defer func() {
				assert.NoError(t, os.Unsetenv("GRADLE_USER_HOME"))
			}()

			// Create a dummy init script
			initScript := "init script content"

			if tt.existingScript {
				existingScriptPath := filepath.Join(tempDir, "init.gradle")
				require.NoError(t, os.WriteFile(existingScriptPath, []byte("existing content"), 0644))
			}

			// Call the function
			err := WriteInitScriptWithBackup(initScript, false)
			assert.NoError(t, err)

			// Verify the init script was written to the correct location
			initScriptPath := filepath.Join(tempDir, "init.gradle")
			content, err := os.ReadFile(initScriptPath)
			assert.NoError(t, err)
			assert.Equal(t, initScript, string(content))

			// Verify backup if there was an existing script
			if tt.expectedBackup {
				backupScriptPath := initScriptPath + ".bak"
				backupContent, err := os.ReadFile(backupScriptPath)
				assert.NoError(t, err)
				assert.Equal(t, "existing content", string(backupContent))
			}
		})
	}
}
