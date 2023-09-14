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
	directDependenciesCves   []string
	xrayResults              []services.ScanResponse
	scanner                  *jas.JasScanner
	thirdPartyScan           bool
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

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, directDependencies []string, scanner *jas.JasScanner, thirdPartyScan bool) (manager *ApplicabilityScanManager) {
	directDependenciesCves := extractDirectDependenciesCvesFromScan(xrayScanResults, directDependencies)
	return &ApplicabilityScanManager{
		applicabilityScanResults: []*sarif.Run{},
		directDependenciesCves:   directDependenciesCves,
		xrayResults:              xrayScanResults,
		scanner:                  scanner,
		thirdPartyScan:           thirdPartyScan,
	}
}

// This function gets a list of xray scan responses that contain direct and indirect vulnerabilities and returns only direct
// vulnerabilities of the scanned project, ignoring indirect vulnerabilities
func extractDirectDependenciesCvesFromScan(xrayScanResults []services.ScanResponse, directDependencies []string) []string {
	directsCves := datastructures.MakeSet[string]()
	for _, scanResult := range xrayScanResults {
		for _, vulnerability := range scanResult.Vulnerabilities {
			if isDirectComponents(maps.Keys(vulnerability.Components), directDependencies) {
				for _, cve := range vulnerability.Cves {
					if cve.Id != "" {
						directsCves.Add(cve.Id)
					}
				}
			}
		}
		for _, violation := range scanResult.Violations {
			if isDirectComponents(maps.Keys(violation.Components), directDependencies) {
				for _, cve := range violation.Cves {
					if cve.Id != "" {
						directsCves.Add(cve.Id)
					}
				}
			}
		}
	}

	return directsCves.ToSlice()
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
	if err = asm.createConfigFile(wd); err != nil {
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
	return len(asm.directDependenciesCves) > 0
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

func (asm *ApplicabilityScanManager) createConfigFile(workingDir string) error {
	skipDirs := jas.SkippedDirs
	if asm.thirdPartyScan {
		log.Info("Including node modules folder in applicability scan")
		skipDirs = removeElementFromSlice(skipDirs, jas.NodeModulesPattern)
	}
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:        []string{workingDir},
				Output:       asm.scanner.ResultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: asm.directDependenciesCves,
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

func removeElementFromSlice(skipDirs []string, element string) []string {
	deleteIndex := slices.Index(skipDirs, element)
	if deleteIndex == -1 {
		return skipDirs
	}
	return slices.Delete(skipDirs, deleteIndex, deleteIndex+1)
}
