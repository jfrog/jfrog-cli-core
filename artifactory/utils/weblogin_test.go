package utils

import (
	"os"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestOpenBrowserWithFallback_CustomBrowserCommand(t *testing.T) {
	// Test with JFROG_CLI_BROWSER_COMMAND environment variable
	testUrl := "https://example.com"
	customCmd := "echo"

	// Set environment variable
	oldValue := os.Getenv(coreutils.BrowserCommand)
	defer func() {
		if oldValue == "" {
			os.Unsetenv(coreutils.BrowserCommand)
		} else {
			os.Setenv(coreutils.BrowserCommand, oldValue)
		}
	}()

	os.Setenv(coreutils.BrowserCommand, customCmd)

	// This should succeed since 'echo' command exists and will just print the URL
	err := openBrowserWithFallback(testUrl)
	assert.NoError(t, err)
}

func TestOpenBrowserWithFallback_BrowserEnvironmentVariable(t *testing.T) {
	// Test with BROWSER environment variable
	testUrl := "https://example.com"

	// Ensure JFROG_CLI_BROWSER_COMMAND is not set
	oldJfrogValue := os.Getenv(coreutils.BrowserCommand)
	defer func() {
		if oldJfrogValue == "" {
			os.Unsetenv(coreutils.BrowserCommand)
		} else {
			os.Setenv(coreutils.BrowserCommand, oldJfrogValue)
		}
	}()
	os.Unsetenv(coreutils.BrowserCommand)

	// Test with BROWSER=none (should fail)
	oldBrowserValue := os.Getenv("BROWSER")
	defer func() {
		if oldBrowserValue == "" {
			os.Unsetenv("BROWSER")
		} else {
			os.Setenv("BROWSER", oldBrowserValue)
		}
	}()

	os.Setenv("BROWSER", "none")
	err := openBrowserWithFallback(testUrl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "browser opening disabled")

	// Test with BROWSER=echo (should succeed)
	os.Setenv("BROWSER", "echo")
	err = openBrowserWithFallback(testUrl)
	assert.NoError(t, err)
}

func TestRunCustomBrowserCommand_EmptyCommand(t *testing.T) {
	err := runCustomBrowserCommand("", "https://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty browser command")
}

func TestRunCustomBrowserCommand_ValidCommand(t *testing.T) {
	// Use 'echo' command which should succeed
	err := runCustomBrowserCommand("echo", "https://example.com")
	assert.NoError(t, err)
}

func TestRunCustomBrowserCommand_CommandWithArgs(t *testing.T) {
	// Test command with arguments
	err := runCustomBrowserCommand("echo test", "https://example.com")
	assert.NoError(t, err)
}

func TestRunCustomBrowserCommand_NonExistentCommand(t *testing.T) {
	// Test with a command that doesn't exist
	err := runCustomBrowserCommand("nonexistentcommand12345", "https://example.com")
	assert.Error(t, err)
	// The error should be related to command not found
	assert.True(t, strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no such file") ||
		strings.Contains(err.Error(), "executable file not found"))
}

// TestOpenBrowserWithFallback_FallbackToPkgBrowser tests the fallback to pkg/browser
// when no environment variables are set. This test might fail in CI environments
// where no browser is available, so we just check that it attempts the fallback.
func TestOpenBrowserWithFallback_FallbackToPkgBrowser(t *testing.T) {
	testUrl := "https://example.com"

	// Ensure both environment variables are not set
	oldJfrogValue := os.Getenv(coreutils.BrowserCommand)
	oldBrowserValue := os.Getenv("BROWSER")

	defer func() {
		if oldJfrogValue == "" {
			os.Unsetenv(coreutils.BrowserCommand)
		} else {
			os.Setenv(coreutils.BrowserCommand, oldJfrogValue)
		}
		if oldBrowserValue == "" {
			os.Unsetenv("BROWSER")
		} else {
			os.Setenv("BROWSER", oldBrowserValue)
		}
	}()

	os.Unsetenv(coreutils.BrowserCommand)
	os.Unsetenv("BROWSER")

	// This will likely fail in CI environments, but that's expected
	// We're just testing that it attempts to use pkg/browser
	err := openBrowserWithFallback(testUrl)
	// We don't assert on the error since it depends on the environment
	// The important thing is that the function doesn't panic
	t.Logf("Fallback to pkg/browser result: %v", err)
}
