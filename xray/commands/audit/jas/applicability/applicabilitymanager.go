package applicability

import (
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const (
	applicabilityScanType    = "analyze-applicability"
	applicabilityScanCommand = "ca"
)

type ApplicabilityScanManager struct {
	applicabilityScanResults []*sarif.Run
	dependencyWhitelist      []string
	xrayResults              []services.ScanResponse
	scanner                  *jas.JasScanner
	// Include third party dependencies source code in the scan
	thirdPartyContextualAnalysis bool
}

// The getApplicabilityScanResults function runs the applicability scan flow, which includes the following steps:
// Creating an ApplicabilityScanManager object.
// Checking if the scanned project is eligible for applicability scan.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// map[string]string: A map containing the applicability result of each XRAY CVE.
// bool: true if the user is entitled to the applicability scan, false otherwise.
// error: An error object (if any).
func RunApplicabilityScan(xrayResults []services.ScanResponse, directDependencies []string,
	scannedTechnologies []coreutils.Technology, scanner *jas.JasScanner, thirdPartyContextualAnalysis bool) (results []*sarif.Run, err error) {
	applicabilityScanManager := newApplicabilityScanManager(xrayResults, directDependencies, scanner, thirdPartyContextualAnalysis)
	if !applicabilityScanManager.shouldRunApplicabilityScan(scannedTechnologies) {
		log.Debug("The technologies that have been scanned are currently not supported for contextual analysis scanning, or we couldn't find any vulnerable direct dependencies. Skipping....")
		return
	}
	if err = applicabilityScanManager.scanner.Run(applicabilityScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Applicability, err)
		return
	}
	results = applicabilityScanManager.applicabilityScanResults
	return
}

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, directDependencies []string, scanner *jas.JasScanner, thirdPartyContextualAnalysis bool) (manager *ApplicabilityScanManager) {
	dependencyWhitelist := prepareDependenciesCvesWhitelist(xrayScanResults, directDependencies, thirdPartyContextualAnalysis)
	return &ApplicabilityScanManager{
		applicabilityScanResults:     []*sarif.Run{},
		dependencyWhitelist:          dependencyWhitelist,
		xrayResults:                  xrayScanResults,
		scanner:                      scanner,
		thirdPartyContextualAnalysis: thirdPartyContextualAnalysis,
	}
}

// Prepares a list of CVES for the scanner to scan.
// In most cases, we will send only direct dependencies to the whitelist
// Except when ThirdPartyContextualAnalysis is set to true.
func prepareDependenciesCvesWhitelist(xrayScanResults []services.ScanResponse, directDependencies []string, thirdPartyContextualAnalysis bool) []string {
	whitelistCves := datastructures.MakeSet[string]()
	for _, scanResult := range xrayScanResults {
		for _, vulnerability := range scanResult.Vulnerabilities {
			if thirdPartyContextualAnalysis || isDirectComponents(maps.Keys(vulnerability.Components), directDependencies) {
				for _, cve := range vulnerability.Cves {
					if cve.Id != "" {
						whitelistCves.Add(cve.Id)
					}
				}
			}
		}
		for _, violation := range scanResult.Violations {
			if thirdPartyContextualAnalysis || isDirectComponents(maps.Keys(violation.Components), directDependencies) {
				for _, cve := range violation.Cves {
					if cve.Id != "" {
						whitelistCves.Add(cve.Id)
					}
				}
			}
		}
	}

	return whitelistCves.ToSlice()
}

func isDirectComponents(components []string, directDependencies []string) bool {
	for _, component := range components {
		if slices.Contains(directDependencies, component) {
			return true
		}
	}
	return false
}

func (asm *ApplicabilityScanManager) Run(wd string) (err error) {
	if len(asm.scanner.WorkingDirs) > 1 {
		log.Info("Running applicability scanning in the", wd, "directory...")
	} else {
		log.Info("Running applicability scanning...")
	}
	if err = asm.createConfigFile(wd, asm.thirdPartyContextualAnalysis); err != nil {
		return
	}
	if err = asm.runAnalyzerManager(); err != nil {
		return
	}
	workingDirResults, err := jas.ReadJasScanRunsFromFile(asm.scanner.ResultsFileName, wd)
	if err != nil {
		return
	}
	asm.applicabilityScanResults = append(asm.applicabilityScanResults, workingDirResults...)
	return
}

func (asm *ApplicabilityScanManager) directDependenciesExist() bool {
	return len(asm.dependencyWhitelist) > 0
}

func (asm *ApplicabilityScanManager) shouldRunApplicabilityScan(technologies []coreutils.Technology) bool {
	return asm.directDependenciesExist() && coreutils.ContainsApplicabilityScannableTech(technologies)
}

type applicabilityScanConfig struct {
	Scans []scanConfiguration `yaml:"scans"`
}

type scanConfiguration struct {
	Roots        []string `yaml:"roots"`
	Output       string   `yaml:"output"`
	Type         string   `yaml:"type"`
	GrepDisable  bool     `yaml:"grep-disable"`
	CveWhitelist []string `yaml:"cve-whitelist"`
	SkippedDirs  []string `yaml:"skipped-folders"`
}

func (asm *ApplicabilityScanManager) createConfigFile(workingDir string, thirdPartyContextualAnalysis bool) error {
	skipDirs := jas.SkippedDirs
	// If set to true, include third party dependencies code and ignore only test folders.
	if thirdPartyContextualAnalysis {
		skipDirs = []string{"**/*test*/**"}
	}
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:        []string{workingDir},
				Output:       asm.scanner.ResultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: asm.dependencyWhitelist,
				SkippedDirs:  skipDirs,
			},
		},
	}
	return jas.CreateScannersConfigFile(asm.scanner.ConfigFileName, configFileContent)
}

// Runs the analyzerManager app and returns a boolean to indicate whether the user is entitled for
// advance security feature
func (asm *ApplicabilityScanManager) runAnalyzerManager() error {
	return asm.scanner.AnalyzerManager.Exec(asm.scanner.ConfigFileName, applicabilityScanCommand, filepath.Dir(asm.scanner.AnalyzerManager.AnalyzerManagerFullPath), asm.scanner.ServerDetails)
}
