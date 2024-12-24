package usage

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/jfconnect/services"
)

type VisibilitySystemManager struct {
	serverDetails *config.ServerDetails
}

func NewVisibilitySystemManager(serverDetails *config.ServerDetails) *VisibilitySystemManager {
	return &VisibilitySystemManager{
		serverDetails: serverDetails,
	}
}

func (vsm *VisibilitySystemManager) createMetric(commandName string) services.VisibilityMetric {
	metricLabels := services.Labels{
		ProductID:                            coreutils.GetCliUserAgentName(),
		ProductVersion:                       coreutils.GetCliUserAgentVersion(),
		FeatureID:                            commandName,
		OIDCUsed:                             os.Getenv("JFROG_CLI_USAGE_OIDC_USED"),
		JobID:                                os.Getenv("JFROG_CLI_USAGE_JOB_ID"),
		RunID:                                os.Getenv("JFROG_CLI_USAGE_RUN_ID"),
		GitRepo:                              os.Getenv("JFROG_CLI_USAGE_GIT_REPO"),
		GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
	}

	return services.VisibilityMetric{
		Value:       1,
		MetricsName: "jfcli_commands_count",
		Labels:      metricLabels,
	}
}

func (vsm *VisibilitySystemManager) SendUsage(commandName string) error {
	manager, err := utils.CreateJfConnectServiceManager(vsm.serverDetails)
	if err != nil {
		return err
	}
	return manager.PostVisibilityMetric(vsm.createMetric(commandName))
}
