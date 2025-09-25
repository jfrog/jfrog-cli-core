package visibility

import (
	"encoding/json"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/general/token"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommandsCountMetric(t *testing.T) {
	// Set environment variables for the test using SetEnvWithCallbackAndAssert
	envVars := map[string]string{
		coreutils.CIJobID:          "job123",
		coreutils.CIRunID:          "run456",
		coreutils.CIVcsUrl:         "test-repo",
		coreutils.OidcProviderType: token.GitHub.String(),
		"JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED": "TRUE",
	}
	cleanupFuncs := []func(){}
	for key, value := range envVars {
		cleanup := testsutils.SetEnvWithCallbackAndAssert(t, key, value)
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}
	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	commandName := "testCommand"
	metric := NewCommandsCountMetric(commandName)
	metricJSON, err := json.Marshal(metric)
	assert.NoError(t, err)

	// Define the expected JSON structure
	expectedJSON := `{
		"value": 1,
		"metrics_name": "jfcli_commands_count",
		"labels": {
			"product_id": "` + coreutils.GetCliUserAgentName() + `",
			"product_version": "` + coreutils.GetCliUserAgentVersion() + `",
			"feature_id": "testCommand",
			"provider_type": "GitHub",
			"job_id": "job123",
			"run_id": "run456",
			"git_repo": "test-repo",
			"gh_token_for_code_scanning_alerts_provided": "TRUE",
			"flags": "",
			"platform": "",
			"architecture": "",
			"is_ci": "",
			"is_container": ""
		}
	}`

	// Compare the generated JSON to the expected JSON
	assert.JSONEq(t, expectedJSON, string(metricJSON))
}

func TestNewCommandsCountMetricWithEnhancedData(t *testing.T) {
	commandName := "enhanced-test-command"

	metricsData := &MetricsData{
		Flags:        []string{"verbose", "recursive", "threads"},
		Platform:     "linux",
		Architecture: "amd64",
		IsCI:         true,
		CISystem:     "github_actions",
		IsContainer:  false,
	}

	metric := NewCommandsCountMetricWithEnhancedData(commandName, metricsData)

	// Verify basic metric structure
	assert.Equal(t, 1, metric.Value)
	assert.Equal(t, "jfcli_commands_count", metric.Name)

	// Verify labels
	labels, ok := metric.Labels.(*commandsCountLabels)
	assert.True(t, ok, "Expected labels to be of type commandsCountLabels")

	// Verify basic fields
	assert.Equal(t, coreutils.GetCliUserAgentName(), labels.ProductID)
	assert.Equal(t, commandName, labels.FeatureID)

	// Verify enhanced fields
	assert.Equal(t, "recursive,threads,verbose", labels.Flags)
	assert.Equal(t, "linux", labels.Platform)
	assert.Equal(t, "amd64", labels.Architecture)
	assert.Equal(t, "true", labels.IsCI)
	assert.Equal(t, "github_actions", labels.CISystem)
	assert.Equal(t, "false", labels.IsContainer)
}

func TestNewCommandsCountMetricWithNilEnhancedData(t *testing.T) {
	commandName := "nil-enhanced-test-command"

	metric := NewCommandsCountMetricWithEnhancedData(commandName, nil)

	// Verify basic metric structure
	assert.Equal(t, 1, metric.Value)
	assert.Equal(t, "jfcli_commands_count", metric.Name)

	// Verify labels
	labels, ok := metric.Labels.(*commandsCountLabels)
	assert.True(t, ok, "Expected labels to be of type commandsCountLabels")

	// Verify basic fields are still set
	assert.Equal(t, commandName, labels.FeatureID)

	// Verify enhanced fields are empty/default
	assert.Empty(t, labels.Flags)
	assert.Empty(t, labels.Platform)
	assert.Empty(t, labels.Architecture)
	assert.Empty(t, labels.IsCI)
	assert.Empty(t, labels.CISystem)
	assert.Empty(t, labels.IsContainer)
}

func TestMetricsDataStructure(t *testing.T) {
	data := &MetricsData{
		Flags:        []string{"flag1", "flag2"},
		Platform:     "windows",
		Architecture: "arm64",
		IsCI:         false,
		CISystem:     "",
		IsContainer:  true,
	}

	// Verify all fields are accessible
	assert.Len(t, data.Flags, 2)
	assert.Equal(t, "flag1", data.Flags[0])
	assert.Equal(t, "windows", data.Platform)
	assert.Equal(t, "arm64", data.Architecture)
	assert.False(t, data.IsCI)
	assert.Empty(t, data.CISystem)
	assert.True(t, data.IsContainer)
}

func TestCommandsCountLabelsJSONSerialization(t *testing.T) {
	metricsData := &MetricsData{
		Flags:        []string{"recursive", "dry-run"},
		Platform:     "darwin",
		Architecture: "arm64",
		IsCI:         true,
		CISystem:     "github_actions",
		IsContainer:  false,
	}

	commandName := "test-upload"
	metric := NewCommandsCountMetricWithEnhancedData(commandName, metricsData)

	// Serialize to JSON
	metricJSON, err := json.Marshal(metric)
	assert.NoError(t, err)

	// Verify the JSON contains our enhanced fields
	assert.Contains(t, string(metricJSON), "\"flags\":\"dry-run,recursive\"")
	assert.Contains(t, string(metricJSON), "\"platform\":\"darwin\"")
	assert.Contains(t, string(metricJSON), "\"architecture\":\"arm64\"")
	assert.Contains(t, string(metricJSON), "\"is_ci\":\"true\"")
	assert.Contains(t, string(metricJSON), "\"ci_system\":\"github_actions\"")
	assert.Contains(t, string(metricJSON), "\"is_container\":\"false\"")
}

func TestEnhancedMetricsEnvironmentIntegration(t *testing.T) {
	// Set environment variables for the test (using the keys consumed by the code)
	envVars := map[string]string{
		coreutils.CIJobID:              "test-job-123",
		coreutils.CIRunID:              "test-run-456",
		coreutils.SourceCodeRepository: "owner/repo",
	}
	cleanupFuncs := []func(){}
	for key, value := range envVars {
		cleanup := testsutils.SetEnvWithCallbackAndAssert(t, key, value)
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}
	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	metricsData := &MetricsData{
		Flags:        []string{"threads", "retry"},
		Platform:     "linux",
		Architecture: "amd64",
		IsCI:         true,
		CISystem:     "jenkins",
		IsContainer:  true,
	}

	commandName := "env-test-command"
	metric := NewCommandsCountMetricWithEnhancedData(commandName, metricsData)

	labels, ok := metric.Labels.(*commandsCountLabels)
	assert.True(t, ok, "Expected labels to be of type commandsCountLabels")

	// Verify environment variables are picked up
	assert.Equal(t, "", labels.ProviderType)
	assert.Equal(t, "test-job-123", labels.JobID)
	assert.Equal(t, "test-run-456", labels.RunID)
	assert.Equal(t, "owner/repo", labels.GitRepo)

	// Verify enhanced metrics data is also included
	assert.Equal(t, "retry,threads", labels.Flags)
	assert.Equal(t, "linux", labels.Platform)
	assert.Equal(t, "amd64", labels.Architecture)
	assert.Equal(t, "true", labels.IsCI)
	assert.Equal(t, "jenkins", labels.CISystem)
	assert.Equal(t, "true", labels.IsContainer)
}
