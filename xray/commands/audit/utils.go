package audit

import (
	"github.com/jfrog/jfrog-cli-core/xray/commands"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func produceAuditErrorIfNeeded(scanResults *services.ScanResponse) error {
	for _, violation := range scanResults.Violations {
		if violation.FailBuild {
			return &commands.AuditError{}
		}
	}
	return nil
}
