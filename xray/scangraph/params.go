package scangraph

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type ScanGraphParams struct {
	serverDetails       *config.ServerDetails
	xrayGraphScanParams *services.XrayGraphScanParams
	fixableOnly         bool
	xrayVersion         string
	severityLevel       int
}

func NewScanGraphParams() *ScanGraphParams {
	return &ScanGraphParams{}
}

func (sgp *ScanGraphParams) SetServerDetails(serverDetails *config.ServerDetails) *ScanGraphParams {
	sgp.serverDetails = serverDetails
	return sgp
}

func (sgp *ScanGraphParams) SetXrayGraphScanParams(params *services.XrayGraphScanParams) *ScanGraphParams {
	sgp.xrayGraphScanParams = params
	return sgp
}

func (sgp *ScanGraphParams) SetXrayVersion(xrayVersion string) *ScanGraphParams {
	sgp.xrayVersion = xrayVersion
	return sgp
}

func (sgp *ScanGraphParams) SetSeverityLevel(severity string) *ScanGraphParams {
	sgp.severityLevel = getLevelOfSeverity(severity)
	return sgp
}

func (sgp *ScanGraphParams) XrayGraphScanParams() *services.XrayGraphScanParams {
	return sgp.xrayGraphScanParams
}

func (sgp *ScanGraphParams) XrayVersion() string {
	return sgp.xrayVersion
}

func (sgp *ScanGraphParams) ServerDetails() *config.ServerDetails {
	return sgp.serverDetails
}

func (sgp *ScanGraphParams) FixableOnly() bool {
	return sgp.fixableOnly
}

func (sgp *ScanGraphParams) SetFixableOnly(fixable bool) *ScanGraphParams {
	sgp.fixableOnly = fixable
	return sgp
}
