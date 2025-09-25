package visibility

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/general/token"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
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
			"gh_token_for_code_scanning_alerts_provided": "TRUE"
		}
	}`

	// Compare the generated JSON to the expected JSON
	assert.JSONEq(t, expectedJSON, string(metricJSON))
}
