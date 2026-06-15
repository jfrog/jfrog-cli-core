package cliutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	testUtils "github.com/jfrog/jfrog-cli-core/v2/utils/tests"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestReadJFrogApplicationKeyFromConfigOrEnv(t *testing.T) {
	configFilePath := filepath.Join(JfConfigDirName, JfConfigFileName)

	// Test cases
	tests := []struct {
		name           string
		configContent  string
		envValue       string
		expectedResult string
	}{
		{
			name:           "Application key in config file",
			configContent:  "application:\n  key: configKey",
			envValue:       "",
			expectedResult: "configKey",
		},
		{
			name:           "Application key in environment variable",
			configContent:  "",
			envValue:       "envKey",
			expectedResult: "envKey",
		},
		{
			name:           "Application key in both config file and environment variable",
			configContent:  "application:\n  key: configKey",
			envValue:       "envKey",
			expectedResult: "envKey",
		},
		{
			name:           "No application key in config file or environment variable",
			configContent:  "",
			envValue:       "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp dir for each test
			testDirPath, err := filepath.Abs(filepath.Join("../", "tests", "applicationConfigTestDir"))
			assert.NoError(t, err)
			_, cleanUp := testUtils.CreateTestWorkspace(t, testDirPath)

			// Write config content to file
			if tt.configContent != "" {
				err = os.WriteFile(configFilePath, []byte(tt.configContent), 0644)
				assert.NoError(t, err)
			}

			// Set environment variable
			if tt.envValue != "" {
				assert.NoError(t, os.Setenv(coreutils.ApplicationKey, tt.envValue))
			} else {
				assert.NoError(t, os.Unsetenv(coreutils.ApplicationKey))
			}

			// Call the function
			result := ReadJFrogApplicationKeyFromConfigOrEnv()

			// Assert the result
			assert.Equal(t, tt.expectedResult, result)
			// delete temp folder
			cleanUp()
		})
	}
}

func TestShouldOfferConfig_AgentSkipsPrompt(t *testing.T) {
	tests := []struct {
		name        string
		ciEnv       string
		agentEnv    string // CLAUDECODE env var to simulate an agent
		wantOffer   bool
		wantErr     bool
	}{
		{
			name:      "agent env set — no prompt",
			agentEnv:  "1",
			wantOffer: false,
		},
		{
			name:      "CI=true — no prompt",
			ciEnv:     "true",
			wantOffer: false,
		},
		{
			name:      "CI=true and agent set — no prompt",
			ciEnv:     "true",
			agentEnv:  "1",
			wantOffer: false,
		},
		{
			name:      "neither CI nor agent — would prompt (AskYesNo returns false on non-TTY)",
			wantOffer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure no server config exists for the duration of this subtest.
			_, cleanUp := testUtils.CreateTestWorkspace(t, t.TempDir())
			defer cleanUp()

			// Reset the memoized execution context so our env var changes take effect.
			commands.ResetExecutionContextForTest()
			defer commands.ResetExecutionContextForTest()

			if tt.ciEnv != "" {
				assert.NoError(t, os.Setenv(coreutils.CI, tt.ciEnv))
				defer func() { _ = os.Unsetenv(coreutils.CI) }()
			} else {
				_ = os.Unsetenv(coreutils.CI)
			}

			if tt.agentEnv != "" {
				assert.NoError(t, os.Setenv("CLAUDECODE", tt.agentEnv))
				defer func() { _ = os.Unsetenv("CLAUDECODE") }()
			} else {
				_ = os.Unsetenv("CLAUDECODE")
			}

			offer, err := ShouldOfferConfig()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantOffer, offer)
		})
	}
}
