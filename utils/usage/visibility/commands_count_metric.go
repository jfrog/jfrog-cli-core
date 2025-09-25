package visibility

import (
	"os"
	"sort"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	metrics "github.com/jfrog/jfrog-cli-core/v2/utils/metrics"
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
)

// MetricsData is shared from utils/metrics to avoid import cycles.
type MetricsData = metrics.MetricsData

type commandsCountLabels struct {
	ProductID                            string `json:"product_id"`
	ProductVersion                       string `json:"product_version"`
	FeatureID                            string `json:"feature_id"`
	ProviderType                         string `json:"provider_type"`
	JobID                                string `json:"job_id"`
	RunID                                string `json:"run_id"`
	GitRepo                              string `json:"git_repo"`
	GhTokenForCodeScanningAlertsProvided string `json:"gh_token_for_code_scanning_alerts_provided"`
	Flags                                string `json:"flags"`
	Platform                             string `json:"platform"`
	Architecture                         string `json:"architecture"`
	IsCI                                 string `json:"is_ci"`
	CISystem                             string `json:"ci_system,omitempty"`
	IsContainer                          string `json:"is_container"`
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
			Flags:                                "",
			Platform:                             "",
			Architecture:                         "",
			IsCI:                                 "",
			CISystem:                             "",
			IsContainer:                          "",
		},
	}
}

func NewCommandsCountMetricWithEnhancedData(commandName string, metricsData *MetricsData) services.VisibilityMetric {
	metric := NewCommandsCountMetric(commandName)
	labels, _ := metric.Labels.(*commandsCountLabels)

	if metricsData != nil {
		if len(metricsData.Flags) > 0 {
			flags := append([]string(nil), metricsData.Flags...)
			sort.Strings(flags)
			labels.Flags = strings.Join(flags, ",")
		}
		labels.Platform = metricsData.Platform
		labels.Architecture = metricsData.Architecture
		if metricsData.IsCI {
			labels.IsCI = "true"
			labels.CISystem = metricsData.CISystem
		} else {
			labels.IsCI = "false"
			labels.CISystem = ""
		}
		if metricsData.IsContainer {
			labels.IsContainer = "true"
		} else {
			labels.IsContainer = "false"
		}
	}

	return metric
}
