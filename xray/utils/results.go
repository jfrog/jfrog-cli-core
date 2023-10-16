package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

type Results struct {
	ScaResults []ScaScanResult
	XrayVersion         string
	ScaError              error
	
	IsMultipleRootProject bool // TODO: remove this
	ExtendedScanResults   *ExtendedScanResults
	JasError              error
}

func NewAuditResults() *Results {
	return &Results{ExtendedScanResults: &ExtendedScanResults{}}
}

type ScaScanResult struct {
	Technology  coreutils.Technology
	WorkingDirectory string
	Descriptors []string
	XrayResults []services.ScanResponse

	// IsMultipleRootProject bool
}

type ExtendedScanResults struct {
	XrayResults         []services.ScanResponse // TODO: remove this
	XrayVersion         string // TODO: remove this
	ScannedTechnologies []coreutils.Technology // TODO: remove this

	ApplicabilityScanResults []*sarif.Run
	SecretsScanResults       []*sarif.Run
	IacScanResults           []*sarif.Run
	SastScanResults          []*sarif.Run
	EntitledForJas           bool
}

func (e *ExtendedScanResults) getXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}