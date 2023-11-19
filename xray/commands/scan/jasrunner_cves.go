package scan

import (
	"errors"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas/applicability"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas/secrets"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func runJasScannersAndSetResults(scanResults *utils.Results, cveList []string,
	serverDetails *config.ServerDetails, workingDirs []string, progress io.ProgressMgr, multiScanId string, thirdPartyApplicabilityScan bool) (err error) {

	if serverDetails == nil || len(serverDetails.Url) == 0 {
		log.Warn("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add' command before running this command.")
		return
	}

	scanner, err := jas.NewJasScanner(workingDirs, serverDetails, multiScanId)
	if err != nil {
		return
	}

	defer func() {
		cleanup := scanner.ScannerDirCleanupFunc
		err = errors.Join(err, cleanup())
	}()

	// if progress != nil {
	// 	progress.SetHeadlineMsg("Running applicability scanning")
	// }

	scanResults.ExtendedScanResults.ApplicabilityScanResults, err = applicability.RunApplicabilityWithScanCves(scanResults.GetScaScansXrayResults(), cveList, scanResults.GetScaScannedTechnologies(), scanner, thirdPartyApplicabilityScan)
	if err != nil {
		fmt.Println("there was an error:", err)
		return
	}

	// if progress != nil {
	// 	progress.SetHeadlineMsg("Running secrets scanning")
	// }
	scanResults.ExtendedScanResults.SecretsScanResults, err = secrets.RunSecretsScan(scanner, secrets.SecretsScannerDockerType)
	if err != nil {
		return
	}
	return
}
