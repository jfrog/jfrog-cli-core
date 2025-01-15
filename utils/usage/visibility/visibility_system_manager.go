package visibility

import (
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

func (vsm *VisibilitySystemManager) createCommandsCountMetric(commandName string) commandsCountMetric {
	metricLabels := newCommandsCountMetric()
	metricLabels.Labels = commandsCountLabels{
		ProductID:                            coreutils.GetCliUserAgentName(),
		ProductVersion:                       coreutils.GetCliUserAgentVersion(),
		FeatureID:                            commandName,
		OIDCUsed:                             os.Getenv("JFROG_CLI_USAGE_OIDC_USED"),
		JobID:                                os.Getenv("JFROG_CLI_USAGE_JOB_ID"),
		RunID:                                os.Getenv("JFROG_CLI_USAGE_RUN_ID"),
		GitRepo:                              os.Getenv("JFROG_CLI_USAGE_GIT_REPO"),
		GhTokenForCodeScanningAlertsProvided: os.Getenv("JFROG_CLI_USAGE_GH_TOKEN_FOR_CODE_SCANNING_ALERTS_PROVIDED"),
	}
	return metricLabels
}

func (vsm *VisibilitySystemManager) SendUsage(commandName string) error {
	manager, err := utils.CreateJfConnectServiceManager(vsm.serverDetails, 0, 0)
	if err != nil {
		return err
	}
	metric := vsm.createCommandsCountMetric(commandName)
	return manager.PostVisibilityMetric(&metric)
}
