package audit

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
)

var analyzerManagerExecuter utils.AnalyzerManagerInterface = &utils.AnalyzerManager{}

func GetExtendedScanResults(xrayResults []services.ScanResponse, dependencyTrees []*services.GraphNode,
	serverDetails *config.ServerDetails) (*utils.ExtendedScanResults, error) {
	if serverDetails == nil {
		return nil, errors.New("cant get xray server details")
	}
	analyzerManagerExist, err := analyzerManagerExecuter.ExistLocally()
	if err != nil {
		return nil, err
	}
	if !analyzerManagerExist {
		log.Debug("analyzer manager doesnt exist, didnt execute analyzer manager")
		return &utils.ExtendedScanResults{XrayResults: xrayResults}, nil
	}
	if err = utils.CreateAnalyzerManagerLogDir(); err != nil {
		return nil, err
	}
	applicabilityScanResults, eligibleForApplicabilityScan, err := getApplicabilityScanResults(xrayResults,
		dependencyTrees, serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	secretsScanResults, eligibleForSecretsScan, err := getSecretsScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	iacScanResults, eligibleForIacScan, err := getIacScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	return &utils.ExtendedScanResults{
		XrayResults:                  xrayResults,
		ApplicabilityScanResults:     applicabilityScanResults,
		SecretsScanResults:           secretsScanResults,
		IacScanResults:               iacScanResults,
		EntitledForJas:               true,
		EligibleForApplicabilityScan: eligibleForApplicabilityScan,
		EligibleForSecretScan:        eligibleForSecretsScan,
		EligibleForIacScan:           eligibleForIacScan,
	}, nil
}

func deleteJasProcessFiles(configFile string, resultFile string) error {
	exist, err := fileutils.IsFileExists(configFile, false)
	if err != nil {
		return err
	}
	if exist {
		err = os.Remove(configFile)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	exist, err = fileutils.IsFileExists(resultFile, false)
	if err != nil {
		return err
	}
	if exist {
		err = os.Remove(resultFile)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	return nil
}
