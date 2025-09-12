package visibility

import (
	"os"
	"sort"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
)

type MetricsData struct {
	FlagsUsed    []string `json:"flags_used,omitempty"`
	OS           string   `json:"os,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	IsCI         bool     `json:"is_ci,omitempty"`
	CISystem     string   `json:"ci_system,omitempty"`
	IsContainer  bool     `json:"is_container,omitempty"`
}

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
			GitRepo:                              os.Getenv(coreutils.SourceCodeRepository),
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
	labels := &commandsCountLabels{
		ProductID:                            coreutils.GetCliUserAgentName(),
		ProductVersion:                       coreutils.GetCliUserAgentVersion(),
		FeatureID:                            commandName,
		ProviderType:                         os.Getenv(coreutils.OidcProviderType),
		JobID:                                os.Getenv(coreutils.CIJobID),
		RunID:                                os.Getenv(coreutils.CIRunID),
		GitRepo:                              os.Getenv(coreutils.SourceCodeRepository),
		GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
		Flags:                                "",
		Platform:                             "",
		Architecture:                         "",
		IsCI:                                 "",
		CISystem:                             "",
		IsContainer:                          "",
	}

	if metricsData != nil {
		if len(metricsData.FlagsUsed) > 0 {
			flags := append([]string(nil), metricsData.FlagsUsed...)
			sort.Strings(flags)
			labels.Flags = strings.Join(flags, ",")
		}
		labels.Platform = metricsData.OS
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

	return services.VisibilityMetric{
		Value:  1,
		Name:   "jfcli_commands_count",
		Labels: labels,
	}
}
