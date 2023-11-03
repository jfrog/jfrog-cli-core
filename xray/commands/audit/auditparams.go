package audit

import (
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditParams struct {
	xrayGraphScanParams *services.XrayGraphScanParams
	workingDirs         []string
	installFunc         func(tech string) error
	fixableOnly         bool
	minSeverityFilter   string
	*xrayutils.AuditBasicParams
	xrayVersion string
	// Include third party dependencies source code in the applicability scan.
	thirdPartyApplicabilityScan bool
}

func NewAuditParams() *AuditParams {
	return &AuditParams{
		xrayGraphScanParams: &services.XrayGraphScanParams{},
		AuditBasicParams:    &xrayutils.AuditBasicParams{},
	}
}

func (params *AuditParams) InstallFunc() func(tech string) error {
	return params.installFunc
}

func (params *AuditParams) XrayGraphScanParams() *services.XrayGraphScanParams {
	return params.xrayGraphScanParams
}

func (params *AuditParams) WorkingDirs() []string {
	return params.workingDirs
}

func (params *AuditParams) XrayVersion() string {
	return params.xrayVersion
}

func (params *AuditParams) SetXrayGraphScanParams(xrayGraphScanParams *services.XrayGraphScanParams) *AuditParams {
	params.xrayGraphScanParams = xrayGraphScanParams
	return params
}

func (params *AuditParams) SetGraphBasicParams(gbp *xrayutils.AuditBasicParams) *AuditParams {
	params.AuditBasicParams = gbp
	return params
}

func (params *AuditParams) SetWorkingDirs(workingDirs []string) *AuditParams {
	params.workingDirs = workingDirs
	return params
}

func (params *AuditParams) SetInstallFunc(installFunc func(tech string) error) *AuditParams {
	params.installFunc = installFunc
	return params
}

func (params *AuditParams) FixableOnly() bool {
	return params.fixableOnly
}

func (params *AuditParams) SetFixableOnly(fixable bool) *AuditParams {
	params.fixableOnly = fixable
	return params
}

func (params *AuditParams) MinSeverityFilter() string {
	return params.minSeverityFilter
}

func (params *AuditParams) SetMinSeverityFilter(minSeverityFilter string) *AuditParams {
	params.minSeverityFilter = minSeverityFilter
	return params
}

func (params *AuditParams) SetThirdPartyApplicabilityScan(includeThirdPartyDeps bool) *AuditParams {
	params.thirdPartyApplicabilityScan = includeThirdPartyDeps
	return params
}

func (params *AuditParams) SetDepsRepo(depsRepo string) *AuditParams {
	params.AuditBasicParams.SetDepsRepo(depsRepo)
	return params
}
