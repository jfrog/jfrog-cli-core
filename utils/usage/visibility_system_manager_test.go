package usage

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestCreateMetric(t *testing.T) {
	// Set environment variables for the test using SetEnvWithCallbackAndAssert
	envVars := map[string]string{
		"JFROG_CLI_USAGE_OIDC_USED":                                  "true",
		"JFROG_CLI_USAGE_JOB_ID":                                     "job123",
		"JFROG_CLI_USAGE_RUN_ID":                                     "run456",
		"JFROG_CLI_USAGE_GIT_REPO":                                   "test-repo",
		"JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED": "true",
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
	metric, err := NewVisibilitySystemManager(nil).createMetric(commandName)
	assert.NoError(t, err)

	// Define the expected JSON structure
	expectedJSON := `{
		"value": 1,
		"metrics_name": "jfcli_commands_countaa",
		"labels": {
			"product_id": "` + coreutils.GetCliUserAgentName() + `",
			"feature_id": "testCommand",
			"oidc_used": "true",
			"job_id": "job123",
			"run_id": "run456",
			"git_repo": "test-repo",
			"gh_token_for_code_scanning_alerts_provided": "true"
		}
	}`

	// Compare the generated JSON to the expected JSON
	assert.JSONEq(t, expectedJSON, string(metric))
}
