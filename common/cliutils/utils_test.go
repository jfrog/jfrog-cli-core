package cliutils

import (
	testUtils "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestReadJFrogApplicationKeyFromConfigOrEnv(t *testing.T) {
	configFilePath := filepath.Join(JFConfigDirName, JFConfigFileName)

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
			expectedResult: "configKey",
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
