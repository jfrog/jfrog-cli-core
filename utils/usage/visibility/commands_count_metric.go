package visibility

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
	"os"
)

type commandsCountLabels struct {
	ProductID                            string `json:"product_id"`
	ProductVersion                       string `json:"product_version"`
	FeatureID                            string `json:"feature_id"`
	JobID                                string `json:"job_id"`
	RunID                                string `json:"run_id"`
	GitRepo                              string `json:"git_repo"`
	GhTokenForCodeScanningAlertsProvided string `json:"gh_token_for_code_scanning_alerts_provided"`
}

func NewCommandsCountMetric(commandName string) services.VisibilityMetric {
	return services.VisibilityMetric{
		Value: 1,
		Name:  "jfcli_commands_count",
		Labels: &commandsCountLabels{
			ProductID:                            coreutils.GetCliUserAgentName(),
			ProductVersion:                       coreutils.GetCliUserAgentVersion(),
			FeatureID:                            commandName,
			JobID:                                os.Getenv("JFROG_CLI_CI_JOB_ID"),
			RunID:                                os.Getenv("JFROG_CLI_CI_RUN_ID"),
			GitRepo:                              os.Getenv("JFROG_CLI_USAGE_GIT_REPO"),
			GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
		},
	}
}
