package visibility

import (
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
)

type commandsCountLabels struct {
	ProductID                            string `json:"product_id"`
	ProductVersion                       string `json:"product_version"`
	FeatureID                            string `json:"feature_id"`
	OIDCUsed                             string `json:"oidc_used"`
	JobID                                string `json:"job_id"`
	RunID                                string `json:"run_id"`
	GitRepo                              string `json:"git_repo"`
	GhTokenForCodeScanningAlertsProvided string `json:"gh_token_for_code_scanning_alerts_provided"`
}

type commandsCountMetric struct {
	services.Metric `json:",inline"`
	Labels          commandsCountLabels `json:"labels"`
}

func newCommandsCountMetric() commandsCountMetric {
	return commandsCountMetric{
		Metric: services.Metric{
			Value: 1,
			Name:  "jfcli_commands_count",
		},
	}
}

func (ccm *commandsCountMetric) MetricsName() string {
	return ccm.Name
}

func (ccm *commandsCountMetric) Value() int {
	return ccm.Metric.Value
}
