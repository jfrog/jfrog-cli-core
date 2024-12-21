package usage

import (
	"encoding/json"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

type VisibilitySystemManager struct {
	serverDetails *config.ServerDetails
}

func NewVisibilitySystemManager(serverDetails *config.ServerDetails) *VisibilitySystemManager {
	return &VisibilitySystemManager{
		serverDetails: serverDetails,
	}
}

type labels struct {
	ProductID                            string `json:"product_id"`
	FeatureID                            string `json:"feature_id"`
	OIDCUsed                             string `json:"oidc_used"`
	JobID                                string `json:"job_id"`
	RunID                                string `json:"run_id"`
	GitRepo                              string `json:"git_repo"`
	GhTokenForCodeScanningAlertsProvided string `json:"gh_token_for_code_scanning_alerts_provided"`
}

type visibilityMetric struct {
	Value       int    `json:"value"`
	MetricsName string `json:"metrics_name"`
	Labels      labels `json:"labels"`
}

func (vsm *VisibilitySystemManager) createMetric(commandName string) ([]byte, error) {
	metricLabels := labels{
		ProductID:                            coreutils.GetCliUserAgentName(),
		FeatureID:                            commandName,
		OIDCUsed:                             os.Getenv("JFROG_CLI_USAGE_OIDC_USED"),
		JobID:                                os.Getenv("JFROG_CLI_USAGE_JOB_ID"),
		RunID:                                os.Getenv("JFROG_CLI_USAGE_RUN_ID"),
		GitRepo:                              os.Getenv("JFROG_CLI_USAGE_GIT_REPO"),
		GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
	}

	metric := visibilityMetric{
		Value:       1,
		MetricsName: "jfcli_commands_count",
		Labels:      metricLabels,
	}

	return json.Marshal(metric)
}

func (vsm *VisibilitySystemManager) SendUsage(commandName string) error {
	manager, err := utils.CreateJfConnectServiceManager(vsm.serverDetails)
	if err != nil {
		return err
	}
	metric, err := vsm.createMetric(commandName)
	if err != nil {
		return err
	}
	return manager.PostMetric(metric)
}
