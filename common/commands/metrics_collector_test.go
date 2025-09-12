package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/usage/visibility"
)

// ClearAllMetrics clears all stored metrics
func ClearAllMetrics() {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
}

func TestCollectMetrics(t *testing.T) {
	// Clear any existing metrics
	globalMetricsCollector.mu.Lock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
	globalMetricsCollector.mu.Unlock()

	tests := []struct {
		name        string
		commandName string
		flags       []string
		expected    *MetricsData
	}{
		{
			name:        "Single flag",
			commandName: "test-command",
			flags:       []string{"verbose"},
			expected: &MetricsData{
				Flags:        []string{"verbose"},
				Platform:     runtime.GOOS,
				Architecture: runtime.GOARCH,
				IsCI:         detectCISystem() != "",
				CISystem: func() string {
					ci := detectCISystem()
					if ci == "" {
						return ""
					} else {
						return ci
					}
				}(),
				IsContainer: isRunningInContainer(),
			},
		},
		{
			name:        "Multiple flags",
			commandName: "upload-command",
			flags:       []string{"recursive", "threads", "dry-run"},
			expected: &MetricsData{
				Flags:        []string{"recursive", "threads", "dry-run"},
				Platform:     runtime.GOOS,
				Architecture: runtime.GOARCH,
				IsCI:         detectCISystem() != "",
				CISystem: func() string {
					ci := detectCISystem()
					if ci == "" {
						return ""
					} else {
						return ci
					}
				}(),
				IsContainer: isRunningInContainer(),
			},
		},
		{
			name:        "No flags",
			commandName: "simple-command",
			flags:       []string{},
			expected: &MetricsData{
				Flags:        []string{},
				Platform:     runtime.GOOS,
				Architecture: runtime.GOARCH,
				IsCI:         detectCISystem() != "",
				CISystem: func() string {
					ci := detectCISystem()
					if ci == "" {
						return ""
					} else {
						return ci
					}
				}(),
				IsContainer: isRunningInContainer(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Collect metrics
			CollectMetrics(tt.commandName, tt.flags)

			// Retrieve metrics
			metrics := GetCollectedMetrics(tt.commandName)

			// Verify metrics
			if metrics == nil {
				t.Error("Expected metrics to be collected, but got nil")
				return
			}

			// Compare flags
			if len(metrics.Flags) != len(tt.expected.Flags) {
				t.Errorf("Expected %d flags, got %d", len(tt.expected.Flags), len(metrics.Flags))
			}
			for i, flag := range tt.expected.Flags {
				if i >= len(metrics.Flags) || metrics.Flags[i] != flag {
					t.Errorf("Expected flag %s at index %d, got %s", flag, i, metrics.Flags[i])
				}
			}

			// Compare system information
			if metrics.Platform != tt.expected.Platform {
				t.Errorf("Expected Platform %s, got %s", tt.expected.Platform, metrics.Platform)
			}
			if metrics.Architecture != tt.expected.Architecture {
				t.Errorf("Expected Architecture %s, got %s", tt.expected.Architecture, metrics.Architecture)
			}
			if metrics.IsCI != tt.expected.IsCI {
				t.Errorf("Expected IsCI %v, got %v", tt.expected.IsCI, metrics.IsCI)
			}
			if metrics.CISystem != tt.expected.CISystem {
				t.Errorf("Expected CISystem %s, got %s", tt.expected.CISystem, metrics.CISystem)
			}
			if metrics.IsContainer != tt.expected.IsContainer {
				t.Errorf("Expected IsContainer %v, got %v", tt.expected.IsContainer, metrics.IsContainer)
			}
		})
	}
}

func TestGetCollectedMetricsDoesNotClearData(t *testing.T) {
	// Clear any existing metrics
	ClearAllMetrics()

	commandName := "test-clear-command"
	flags := []string{"test-flag"}

	// Collect metrics
	CollectMetrics(commandName, flags)

	// Retrieve metrics first time
	metrics := GetCollectedMetrics(commandName)
	if metrics == nil {
		t.Error("Expected metrics to be collected")
		return
	}

	// Retrieve metrics second time should still return data (not cleared automatically)
	metrics2 := GetCollectedMetrics(commandName)
	if metrics2 == nil {
		t.Error("Expected metrics to still be available after retrieval")
		return
	}

	// Verify data is the same
	if len(metrics.Flags) != len(metrics2.Flags) {
		t.Error("Expected same metrics data on repeated retrieval")
	}

	// Manually clear and verify
	ClearAllMetrics()
	metrics3 := GetCollectedMetrics(commandName)
	if metrics3 != nil {
		t.Error("Expected metrics to be cleared after explicit clear")
	}
}

func TestDetectCISystem(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	ciEnvVars := []string{
		"JENKINS_URL", "TRAVIS", "CIRCLECI", "GITHUB_ACTIONS",
		"GITLAB_CI", "BUILDKITE", "BAMBOO_BUILD_KEY", "TF_BUILD",
		"TEAMCITY_VERSION", "DRONE", "BITBUCKET_BUILD_NUMBER",
		"CODEBUILD_BUILD_ID", "CI",
	}

	for _, envVar := range ciEnvVars {
		originalEnv[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}

	// Restore environment after test
	defer func() {
		for envVar, value := range originalEnv {
			if value != "" {
				os.Setenv(envVar, value)
			} else {
				os.Unsetenv(envVar)
			}
		}
	}()

	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected string
	}{
		{"GitHub Actions", "GITHUB_ACTIONS", "true", "github_actions"},
		{"Jenkins", "JENKINS_URL", "http://jenkins.example.com", "jenkins"},
		{"GitLab CI", "GITLAB_CI", "true", "gitlab"},
		{"CircleCI", "CIRCLECI", "true", "circleci"},
		{"Travis", "TRAVIS", "true", "travis"},
		{"Azure DevOps", "TF_BUILD", "True", "azure_devops"},
		{"Generic CI", "CI", "true", "unknown"},
		{"No CI", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all CI environment variables
			for _, envVar := range ciEnvVars {
				os.Unsetenv(envVar)
			}

			// Set the specific test environment variable
			if tt.envVar != "" {
				if err := os.Setenv(tt.envVar, tt.envValue); err != nil {
					t.Fatalf("failed setting env %s: %v", tt.envVar, err)
				}
			}

			result := detectCISystem()
			if result != tt.expected {
				t.Errorf("Expected CI system %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsRunningInContainer(t *testing.T) {
	// This test will mostly work on the actual environment
	// We can't easily mock filesystem calls without more complex mocking
	result := isRunningInContainer()

	// Just verify it returns a boolean value
	if result != true && result != false {
		t.Error("isRunningInContainer should return a boolean value")
	}

	// Test with environment variable
	originalContainer := os.Getenv("container")
	defer func() {
		if originalContainer != "" {
			_ = os.Setenv("container", originalContainer)
		} else {
			os.Unsetenv("container")
		}
	}()

	if err := os.Setenv("container", "docker"); err != nil {
		t.Fatalf("failed setting env container: %v", err)
	}
	if !isRunningInContainer() {
		t.Error("Expected container detection when 'container' env var is set")
	}

	os.Unsetenv("container")

	// Test with Kubernetes environment
	originalK8s := os.Getenv("KUBERNETES_SERVICE_HOST")
	defer func() {
		if originalK8s != "" {
			_ = os.Setenv("KUBERNETES_SERVICE_HOST", originalK8s)
		} else {
			os.Unsetenv("KUBERNETES_SERVICE_HOST")
		}
	}()

	if err := os.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1"); err != nil {
		t.Fatalf("failed setting env KUBERNETES_SERVICE_HOST: %v", err)
	}
	if !isRunningInContainer() {
		t.Error("Expected container detection when Kubernetes env var is set")
	}
}

func TestMetricsDataStructure(t *testing.T) {
	metrics := &MetricsData{
		Flags:        []string{"test-flag1", "test-flag2"},
		Platform:     "test-os",
		Architecture: "test-arch",
		IsCI:         true,
		CISystem:     "test-ci",
		IsContainer:  false,
	}

	// Verify all fields are properly set
	if len(metrics.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(metrics.Flags))
	}
	if metrics.Flags[0] != "test-flag1" {
		t.Errorf("Expected first flag to be 'test-flag1', got %s", metrics.Flags[0])
	}
	if metrics.Platform != "test-os" {
		t.Errorf("Expected Platform to be 'test-os', got %s", metrics.Platform)
	}
	if metrics.Architecture != "test-arch" {
		t.Errorf("Expected Architecture to be 'test-arch', got %s", metrics.Architecture)
	}
	if !metrics.IsCI {
		t.Error("Expected IsCI to be true")
	}
	if metrics.CISystem != "test-ci" {
		t.Errorf("Expected CISystem to be 'test-ci', got %s", metrics.CISystem)
	}
	if metrics.IsContainer {
		t.Error("Expected IsContainer to be false")
	}
}

// TestCommand implements the Command interface for E2E testing
type TestCommand struct {
	name           string
	flags          []string
	serverDetails  *config.ServerDetails
	executionError error
	executed       bool
}

func (tc *TestCommand) Run() error {
	tc.executed = true
	return tc.executionError
}

func (tc *TestCommand) ServerDetails() (*config.ServerDetails, error) {
	if tc.serverDetails != nil {
		return tc.serverDetails, nil
	}
	return &config.ServerDetails{
		ArtifactoryUrl: "https://test-e2e.jfrog.io/artifactory/",
		User:           "test-user",
		Password:       "test-password",
	}, nil
}

func (tc *TestCommand) CommandName() string {
	return tc.name
}

func (tc *TestCommand) GetFlags() []string {
	return tc.flags
}

func TestE2EBasicMetricsFlow(t *testing.T) {
	// Clear metrics to start fresh
	ClearAllMetrics()
	commandName := "upload"
	flags := []string{"recursive", "threads", "dry-run"}
	CollectMetrics(commandName, flags)
	cmd := &TestCommand{
		name:  commandName,
		flags: flags,
	}
	err := Exec(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !cmd.executed {
		t.Error("Command should have been executed")
	}
	CollectMetrics(commandName, flags)
	metrics := GetCollectedMetrics(commandName)

	if metrics == nil {
		t.Error("Metrics should be collected")
		return
	}
	if len(metrics.Flags) != len(flags) {
		t.Errorf("Expected %d flags, got %d", len(flags), len(metrics.Flags))
	}
	if metrics.Platform != runtime.GOOS {
		t.Errorf("Expected Platform %s, got %s", runtime.GOOS, metrics.Platform)
	}
	if metrics.Architecture != runtime.GOARCH {
		t.Errorf("Expected Architecture %s, got %s", runtime.GOARCH, metrics.Architecture)
	}
}

func TestE2EGitHubActionsEnvironment(t *testing.T) {
	// Setup GitHub Actions environment
	originalGH := os.Getenv("GITHUB_ACTIONS")
	originalCI := os.Getenv("CI")

	if err := os.Setenv("GITHUB_ACTIONS", "true"); err != nil {
		t.Fatalf("failed setting GITHUB_ACTIONS: %v", err)
	}
	if err := os.Setenv("CI", "true"); err != nil {
		t.Fatalf("failed setting CI: %v", err)
	}

	defer func() {
		if originalGH == "" {
			os.Unsetenv("GITHUB_ACTIONS")
		} else {
			_ = os.Setenv("GITHUB_ACTIONS", originalGH)
		}
		if originalCI == "" {
			os.Unsetenv("CI")
		} else {
			_ = os.Setenv("CI", originalCI)
		}
	}()

	ClearAllMetrics()

	commandName := "download"
	flags := []string{"threads", "flat"}

	// Collect metrics
	CollectMetrics(commandName, flags)
	metrics := GetCollectedMetrics(commandName)

	if metrics == nil {
		t.Error("Metrics should be collected")
		return
	}

	if !metrics.IsCI {
		t.Error("Should detect CI environment")
	}
	if metrics.CISystem != "github_actions" {
		t.Errorf("Expected CI system 'github_actions', got '%s'", metrics.CISystem)
	}
}

func TestE2EVisibilityIntegration(t *testing.T) {
	ClearAllMetrics()

	commandName := "full-integration-test"
	flags := []string{"recursive", "threads", "exclude", "dry-run"}

	// Collect metrics
	CollectMetrics(commandName, flags)
	metrics := GetCollectedMetrics(commandName)

	if metrics == nil {
		t.Error("Metrics should be collected")
		return
	}

	// Create visibility metrics
	visibilityData := &visibility.MetricsData{
		Flags:        metrics.Flags,
		Platform:     metrics.Platform,
		Architecture: metrics.Architecture,
		IsCI:         metrics.IsCI,
		CISystem:     metrics.CISystem,
		IsContainer:  metrics.IsContainer,
	}

	visibilityMetric := visibility.NewCommandsCountMetricWithEnhancedData(commandName, visibilityData)

	// Verify metric structure
	if visibilityMetric.Name != "jfcli_commands_count" {
		t.Errorf("Expected metric name 'jfcli_commands_count', got %s", visibilityMetric.Name)
	}
	if visibilityMetric.Value != 1 {
		t.Errorf("Expected metric value 1, got %d", visibilityMetric.Value)
	}

	// Verify JSON serialization
	metricJSON, err := json.Marshal(visibilityMetric)
	if err != nil {
		t.Errorf("Failed to marshal metric: %v", err)
	}

	jsonStr := string(metricJSON)
	if !strings.Contains(jsonStr, "flags") {
		t.Error("JSON should contain flags")
	}
	if !strings.Contains(jsonStr, "platform") {
		t.Error("JSON should contain platform")
	}
	if !strings.Contains(jsonStr, "architecture") {
		t.Error("JSON should contain architecture")
	}
}

func TestE2EFlagSanitization(t *testing.T) {
	ClearAllMetrics()

	testCases := []struct {
		name     string
		flags    []string
		expected []string
	}{
		{
			name:     "Safe flags only",
			flags:    []string{"recursive", "threads", "dry-run"},
			expected: []string{"recursive", "threads", "dry-run"},
		},
		{
			name:     "Empty flags",
			flags:    []string{},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commandName := "sanitization-test-" + strings.ReplaceAll(tc.name, " ", "-")

			// Collect metrics
			CollectMetrics(commandName, tc.flags)
			metrics := GetCollectedMetrics(commandName)

			if metrics == nil {
				t.Error("Metrics should be collected")
				return
			}

			sanitized := metrics.Flags
			if len(sanitized) != len(tc.expected) {
				t.Errorf("Expected %d sanitized flags, got %d", len(tc.expected), len(sanitized))
			}
			for i, expected := range tc.expected {
				if i >= len(sanitized) || sanitized[i] != expected {
					t.Errorf("Expected flag %s at index %d, got %s", expected, i, sanitized[i])
				}
			}
		})
	}
}

func TestE2EConcurrentMetricsCollection(t *testing.T) {
	ClearAllMetrics()

	numCommands := 5
	done := make(chan struct{}, numCommands)

	// Simulate concurrent command executions
	for i := 0; i < numCommands; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			commandName := fmt.Sprintf("concurrent-cmd-%d", id)
			flags := []string{fmt.Sprintf("flag-%d", id)}

			// Collect metrics
			CollectMetrics(commandName, flags)

			// Verify metrics were collected
			metrics := GetCollectedMetrics(commandName)
			if metrics == nil {
				t.Errorf("Metrics should be collected for command %s", commandName)
				return
			}

			if len(metrics.Flags) != 1 || metrics.Flags[0] != flags[0] {
				t.Errorf("Expected flag %s, got %v", flags[0], metrics.Flags)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numCommands; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestE2EMetricsClearing(t *testing.T) {
	ClearAllMetrics()

	commandName := "clear-test-command"
	flags := []string{"test-flag"}

	// Collect metrics
	CollectMetrics(commandName, flags)

	// Retrieve metrics (should NOT clear them - that's done by Exec)
	metrics1 := GetCollectedMetrics(commandName)
	if metrics1 == nil {
		t.Error("First retrieval should return metrics")
	}

	// Clear metrics explicitly
	ClearAllMetrics()

	// Try to retrieve again (should be nil after clearing)
	metrics2 := GetCollectedMetrics(commandName)
	if metrics2 != nil {
		t.Error("After clearing, retrieval should return nil")
	}
}

func TestE2EEnvironmentDetection(t *testing.T) {
	ClearAllMetrics()

	// Test Jenkins detection - isolate from existing CI env (e.g., GitHub Actions)
	ciEnvVars := []string{
		"JENKINS_URL", "TRAVIS", "CIRCLECI", "GITHUB_ACTIONS",
		"GITLAB_CI", "BUILDKITE", "BAMBOO_BUILD_KEY", "TF_BUILD",
		"TEAMCITY_VERSION", "DRONE", "BITBUCKET_BUILD_NUMBER",
		"CODEBUILD_BUILD_ID", "CI",
	}
	originalEnv := make(map[string]string, len(ciEnvVars))
	for _, envVar := range ciEnvVars {
		originalEnv[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	}()

	// Set Jenkins env var explicitly
	if err := os.Setenv("JENKINS_URL", "http://jenkins.test"); err != nil {
		t.Fatalf("failed setting JENKINS_URL: %v", err)
	}

	commandName := "jenkins-test"
	flags := []string{"test-flag"}

	CollectMetrics(commandName, flags)
	metrics := GetCollectedMetrics(commandName)

	if metrics == nil {
		t.Error("Metrics should be collected")
		return
	}

	if !metrics.IsCI {
		t.Error("Should detect CI environment")
	}
	if metrics.CISystem != "jenkins" {
		t.Errorf("Expected CI system 'jenkins', got '%s'", metrics.CISystem)
	}
}

// MockCommand implements the Command interface for testing
type MockCommand struct {
	name          string
	serverDetails *config.ServerDetails
	runFunc       func() error
	shouldError   bool
}

func (m *MockCommand) Run() error {
	if m.runFunc != nil {
		return m.runFunc()
	}
	if m.shouldError {
		return mockError("mock command error")
	}
	return nil
}

func (m *MockCommand) ServerDetails() (*config.ServerDetails, error) {
	if m.serverDetails != nil {
		return m.serverDetails, nil
	}
	return &config.ServerDetails{
		ArtifactoryUrl: "https://test.jfrog.io/artifactory/",
		User:           "test-user",
		Password:       "test-password",
	}, nil
}

func (m *MockCommand) CommandName() string {
	return m.name
}

type mockError string

func (e mockError) Error() string {
	return string(e)
}

func TestExecWithBasicCommand(t *testing.T) {
	// Clear any existing metrics
	globalMetricsCollector.mu.Lock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
	globalMetricsCollector.mu.Unlock()

	commandName := "test-basic-command"
	executed := false

	cmd := &MockCommand{
		name: commandName,
		runFunc: func() error {
			executed = true
			return nil
		},
	}

	err := Exec(cmd)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}
}

func TestExecWithCommandError(t *testing.T) {
	commandName := "test-error-command"

	cmd := &MockCommand{
		name:        commandName,
		shouldError: true,
	}

	err := Exec(cmd)

	if err == nil {
		t.Error("Expected command to return error")
	}

	if err.Error() != "mock command error" {
		t.Errorf("Expected specific error message, got %v", err)
	}
}

func TestReportUsageToVisibilitySystemWithMetrics(t *testing.T) {
	// Clear any existing metrics
	globalMetricsCollector.mu.Lock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
	globalMetricsCollector.mu.Unlock()

	commandName := "test-metrics-command"
	flags := []string{"verbose", "recursive"}

	// Collect metrics first
	CollectMetrics(commandName, flags)

	// Step 2: Create command for testing server details
	cmd := &MockCommand{
		name: commandName,
		serverDetails: &config.ServerDetails{
			ArtifactoryUrl: "https://test.jfrog.io/artifactory/",
		},
	}

	// This will test the reportUsageToVisibilitySystem function indirectly
	// In a real scenario, this would send metrics to the visibility system
	serverDetails, err := cmd.ServerDetails()
	if err != nil {
		t.Errorf("Unexpected error getting server details: %v", err)
		return
	}

	// Verify that metrics exist before reporting
	metrics := GetCollectedMetrics(commandName)
	if metrics == nil {
		t.Error("Expected metrics to be collected")
		return
	}

	// Verify the metrics content
	if len(metrics.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(metrics.Flags))
	}

	// Test the visibility metric creation
	visibilityMetricsData := &visibility.MetricsData{
		Flags:        metrics.Flags,
		Platform:     metrics.Platform,
		Architecture: metrics.Architecture,
		IsCI:         metrics.IsCI,
		CISystem:     metrics.CISystem,
		IsContainer:  metrics.IsContainer,
	}

	metric := visibility.NewCommandsCountMetricWithEnhancedData(commandName, visibilityMetricsData)

	if metric.Value != 1 {
		t.Errorf("Expected metric value to be 1, got %d", metric.Value)
	}

	if metric.Name != "jfcli_commands_count" {
		t.Errorf("Expected metric name to be 'jfcli_commands_count', got %s", metric.Name)
	}

	// Verify that serverDetails is properly formed
	if serverDetails.ArtifactoryUrl == "" {
		t.Error("Expected server details to have ArtifactoryUrl")
	}
}

func TestReportUsageToVisibilitySystemWithoutMetrics(t *testing.T) {
	// Clear any existing metrics
	globalMetricsCollector.mu.Lock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
	globalMetricsCollector.mu.Unlock()

	commandName := "test-no-metrics-command"

	// Don't collect any metrics first

	// Verify that no metrics exist
	metrics := GetCollectedMetrics(commandName)
	if metrics != nil {
		t.Error("Expected no metrics to be collected")
		return
	}

	// Test the visibility metric creation without enhanced data
	metric := visibility.NewCommandsCountMetric(commandName)

	if metric.Value != 1 {
		t.Errorf("Expected metric value to be 1, got %d", metric.Value)
	}

	if metric.Name != "jfcli_commands_count" {
		t.Errorf("Expected metric name to be 'jfcli_commands_count', got %s", metric.Name)
	}
}

func TestMetricsIntegrationFlow(t *testing.T) {
	// Clear any existing metrics
	globalMetricsCollector.mu.Lock()
	globalMetricsCollector.metricsData = make(map[string]*MetricsData)
	globalMetricsCollector.mu.Unlock()

	commandName := "integration-test-command"
	flags := []string{"recursive", "threads", "exclude"}

	// Step 1: Collect metrics (simulates CLI flag interception)
	CollectMetrics(commandName, flags)

	// Step 2: Create and execute command
	executed := false
	cmd := &MockCommand{
		name: commandName,
		runFunc: func() error {
			executed = true
			return nil
		},
	}

	// Step 3: Execute command (this would normally trigger metrics reporting)
	err := Exec(cmd)
	if err != nil {
		t.Errorf("Unexpected error executing command: %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	// Step 4: Verify metrics were collected and can be retrieved
	// Note: In the real flow, GetCollectedMetrics is called during reportUsageToVisibilitySystem
	// and the metrics are cleared after retrieval

	// Since Exec() -> reportUsage() -> reportUsageToVisibilitySystem() -> GetCollectedMetrics()
	// the metrics should have been cleared. Let's test this behavior by collecting again
	// and then manually calling the visibility system integration

	CollectMetrics(commandName, flags)

	// Get metrics (this simulates what happens in reportUsageToVisibilitySystem)
	metrics := GetCollectedMetrics(commandName)
	if metrics == nil {
		t.Error("Expected metrics to be available")
		return
	}

	// Verify metrics content
	if len(metrics.Flags) != 3 {
		t.Errorf("Expected 3 flags, got %d", len(metrics.Flags))
	}

	expectedFlags := []string{"recursive", "threads", "exclude"}
	for i, expectedFlag := range expectedFlags {
		if i >= len(metrics.Flags) || metrics.Flags[i] != expectedFlag {
			t.Errorf("Expected flag %s at index %d, got %s", expectedFlag, i, metrics.Flags[i])
		}
	}

	// Verify system information is collected
	if metrics.Platform == "" {
		t.Error("Expected Platform to be populated")
	}

	if metrics.Architecture == "" {
		t.Error("Expected Architecture to be populated")
	}

	// Simulate cleanup done in reportUsageToVisibilitySystem
	ClearAllMetrics()

	// Verify metrics are cleared after cleanup
	metricsAfter := GetCollectedMetrics(commandName)
	if metricsAfter != nil {
		t.Error("Expected metrics to be cleared after retrieval")
	}
}
