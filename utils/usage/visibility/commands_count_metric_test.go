package visibility

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateCommandsCountMetric(t *testing.T) {
	// Set environment variables for the test using SetEnvWithCallbackAndAssert
	envVars := map[string]string{
		"JFROG_CLI_USAGE_OIDC_USED":                                  "TRUE",
		"JFROG_CLI_USAGE_JOB_ID":                                     "job123",
		"JFROG_CLI_USAGE_RUN_ID":                                     "run456",
		"JFROG_CLI_USAGE_GIT_REPO":                                   "test-repo",
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
	metric := newCommandsCountMetric(commandName)
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
			"oidc_used": "TRUE",
			"job_id": "job123",
			"run_id": "run456",
			"git_repo": "test-repo",
			"gh_token_for_code_scanning_alerts_provided": "TRUE"
		}
	}`

	// Compare the generated JSON to the expected JSON
	assert.JSONEq(t, expectedJSON, string(metricJSON))
}
