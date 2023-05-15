package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

var analyzerManagerExecuter utils.AnalyzerManagerInterface = &utils.AnalyzerManager{}

func GetExtendedScanResults(results []services.ScanResponse, dependencyTrees []*services.GraphNode,
	serverDetails *config.ServerDetails) (*utils.ExtendedScanResults, error) {
	analyzerManagerExist, err := analyzerManagerExecuter.ExistLocally()
	if err != nil {
		return nil, err
	}
	if !analyzerManagerExist {
		log.Debug("analyzer manager doesnt exist, didnt exec analyzer manager\"")
		return &utils.ExtendedScanResults{XrayResults: results}, nil
	}
	if err = utils.CreateAnalyzerManagerLogDir(); err != nil {
		return nil, err
	}
	applicabilityScanResults, eligibleForApplicabilityScan, err := audit.GetApplicabilityScanResults(results,
		dependencyTrees, serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	secretsScanResults, eligibleForSecretsScan, err := audit.getSecretsScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	iacScanResults, eligibleForIacScan, err := audit.GetIacScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	return &ExtendedScanResults{
		XrayResults:                  results,
		ApplicabilityScannerResults:  applicabilityScanResults,
		SecretsScanResults:           secretsScanResults,
		IacScanResults:               iacScanResults,
		EntitledForJas:               true,
		EligibleForApplicabilityScan: eligibleForApplicabilityScan,
		EligibleForSecretScan:        eligibleForSecretsScan,
		EligibleForIacScan:           eligibleForIacScan,
	}, nil
}
