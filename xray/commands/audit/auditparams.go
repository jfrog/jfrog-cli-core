package audit

import (
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/scan"
)

type AuditParams struct {
	xrayGraphScanParams *scan.XrayGraphScanParams
	workingDirs         []string
	installFunc         func(tech string) error
	fixableOnly         bool
	minSeverityFilter   string
	*xrayutils.GraphBasicParams
	xrayVersion string
	xscVersion  string
}

func NewAuditParams() *AuditParams {
	return &AuditParams{
		xrayGraphScanParams: &scan.XrayGraphScanParams{},
		GraphBasicParams:    &xrayutils.GraphBasicParams{},
	}
}

func (params *AuditParams) InstallFunc() func(tech string) error {
	return params.installFunc
}

func (params *AuditParams) XrayGraphScanParams() *scan.XrayGraphScanParams {
	return params.xrayGraphScanParams
}

func (params *AuditParams) WorkingDirs() []string {
	return params.workingDirs
}

func (params *AuditParams) XrayVersion() string {
	return params.xrayVersion
}

func (params *AuditParams) SetXrayGraphScanParams(xrayGraphScanParams *scan.XrayGraphScanParams) *AuditParams {
	params.xrayGraphScanParams = xrayGraphScanParams
	return params
}

func (params *AuditParams) SetGraphBasicParams(gbp *xrayutils.GraphBasicParams) *AuditParams {
	params.GraphBasicParams = gbp
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
