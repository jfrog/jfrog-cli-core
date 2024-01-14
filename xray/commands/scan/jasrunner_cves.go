package scan

import (
	"errors"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas/applicability"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas/secrets"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func runJasScannersAndSetResults(scanResults *utils.Results, cveList []string,
	serverDetails *config.ServerDetails, workingDirs []string) (err error) {

	if serverDetails == nil || len(serverDetails.Url) == 0 {
		log.Warn("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add' command before running this command.")
		return
	}
	multiScanId := "" // Also empty for audit
	scanner, err := jas.NewJasScanner(workingDirs, serverDetails, multiScanId)
	if err != nil {
		return
	}

	defer func() {
		cleanup := scanner.ScannerDirCleanupFunc
		err = errors.Join(err, cleanup())
	}()

	scanResults.ExtendedScanResults.ApplicabilityScanResults, err = applicability.RunApplicabilityWithScanCves(scanResults.GetScaScansXrayResults(), cveList, scanResults.GetScaScannedTechnologies(), scanner)
	if err != nil {
		fmt.Println("there was an error:", err)
		return
	}

	scanResults.ExtendedScanResults.SecretsScanResults, err = secrets.RunSecretsScan(scanner, secrets.SecretsScannerDockerScanType)
	if err != nil {
		return
	}
	return
}
