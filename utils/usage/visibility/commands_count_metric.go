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
	ProviderType                         string `json:"provider_type"`
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
			ProviderType:                         os.Getenv(coreutils.OidcProviderType),
			JobID:                                os.Getenv(coreutils.CIJobID),
			RunID:                                os.Getenv(coreutils.CIRunID),
			GitRepo:                              os.Getenv(coreutils.CIVcsUrl),
			GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
		},
	}
}
