package audit

import (
	"errors"
	"fmt"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/sync/errgroup"
	"os"

	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
)

type AuditCommand struct {
	watches                []string
	projectKey             string
	targetRepoPath         string
	IncludeVulnerabilities bool
	IncludeLicenses        bool
	Fail                   bool
	PrintExtendedTable     bool
	AuditParams
}

type XrayEntitlements struct {
	errGroup *errgroup.Group
	Jas      bool
	Xsc      bool
}

func NewGenericAuditCommand() *AuditCommand {
	return &AuditCommand{AuditParams: *NewAuditParams()}
}

func (auditCmd *AuditCommand) SetWatches(watches []string) *AuditCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditCommand) SetProject(project string) *AuditCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditCommand) SetTargetRepoPath(repoPath string) *AuditCommand {
	auditCmd.targetRepoPath = repoPath
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeVulnerabilities(include bool) *AuditCommand {
	auditCmd.IncludeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeLicenses(include bool) *AuditCommand {
	auditCmd.IncludeLicenses = include
	return auditCmd
}

func (auditCmd *AuditCommand) SetFail(fail bool) *AuditCommand {
	auditCmd.Fail = fail
	return auditCmd
}

func (auditCmd *AuditCommand) SetPrintExtendedTable(printExtendedTable bool) *AuditCommand {
	auditCmd.PrintExtendedTable = printExtendedTable
	return auditCmd
}

func (auditCmd *AuditCommand) CreateXrayGraphScanParams() *services.XrayGraphScanParams {
	params := &services.XrayGraphScanParams{
		RepoPath:    auditCmd.targetRepoPath,
		Watches:     auditCmd.watches,
		ScanType:    services.Dependency,
	}
	if auditCmd.projectKey == "" {
		params.ProjectKey = os.Getenv(coreutils.Project)
	} else {
		params.ProjectKey = auditCmd.projectKey
	}
	params.IncludeVulnerabilities = auditCmd.IncludeVulnerabilities
	params.IncludeLicenses = auditCmd.IncludeLicenses
	return params
}

func (auditCmd *AuditCommand) Run() (err error) {
	workingDirs, err := coreutils.GetFullPathsWorkingDirs(auditCmd.workingDirs)
	if err != nil {
		return
	}
	auditParams := NewAuditParams().
		SetXrayGraphScanParams(auditCmd.CreateXrayGraphScanParams()).
		SetWorkingDirs(workingDirs).
		SetMinSeverityFilter(auditCmd.minSeverityFilter).
		SetFixableOnly(auditCmd.fixableOnly).
		SetGraphBasicParams(auditCmd.AuditBasicParams)
	auditResults, err := RunAudit(auditParams)
	if err != nil {
		return
	}
	if auditCmd.Progress() != nil {
		if err = auditCmd.Progress().Quit(); err != nil {
			return
		}
	}
	var messages []string
	if !auditResults.ExtendedScanResults.EntitledForJas {
		messages = []string{coreutils.PrintTitle("The ‘jf audit’ command also supports JFrog Advanced Security features, such as 'Contextual Analysis', 'Secret Detection', 'IaC Scan' and ‘SAST’.\nThis feature isn't enabled on your system. Read more - ") + coreutils.PrintLink("https://jfrog.com/xray/")}
	}
	// Print Scan results on all cases except if errors accrued on SCA scan and no security/license issues found.
	printScanResults := !(auditResults.ScaError != nil && xrayutils.IsEmptyScanResponse(auditResults.ExtendedScanResults.XrayResults))
	if printScanResults {
		err = xrayutils.PrintScanResults(auditResults.ExtendedScanResults,
			nil,
			auditCmd.OutputFormat(),
			auditCmd.IncludeVulnerabilities,
			auditCmd.IncludeLicenses,
			auditResults.IsMultipleRootProject,
			auditCmd.PrintExtendedTable, false, messages,
		)
		if err != nil {
			return
		}
	}
	if err = errors.Join(auditResults.ScaError, auditResults.JasError); err != nil {
		return
	}

	// Only in case Xray's context was given (!auditCmd.IncludeVulnerabilities), and the user asked to fail the build accordingly, do so.
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities && xrayutils.CheckIfFailBuild(auditResults.ExtendedScanResults.XrayResults) {
		err = xrayutils.NewFailBuildError()
	}
	return
}

func (auditCmd *AuditCommand) CommandName() string {
	return "generic_audit"
}

type Results struct {
	IsMultipleRootProject bool
	ScaError              error
	JasError              error
	ExtendedScanResults   *xrayutils.ExtendedScanResults
	ScannedTechnologies   []coreutils.Technology
}

func NewAuditResults() *Results {
	return &Results{ExtendedScanResults: &xrayutils.ExtendedScanResults{}}
}

// Runs an audit scan based on the provided auditParams.
// Returns an audit Results object containing all the scan results.
// If the current server is entitled for JAS, the advanced security results will be included in the scan results.
func RunAudit(auditParams *AuditParams) (results *Results, err error) {
	var entitlements *XrayEntitlements
	var serverDetails *config.ServerDetails

	// Initialize Results struct
	results = NewAuditResults()
	if serverDetails, err = auditParams.ServerDetails(); err != nil {
		return
	}
	// Check entitlements for JAS and XSC and update auditParams with results.
	if entitlements, err = checkEntitlements(serverDetails, auditParams); err != nil {
		return
	}
	// The sca scan doesn't require the analyzer manager, so it can run separately from the analyzer manager download routine.
	results.ScaError = runScaScan(auditParams, results)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = entitlements.errGroup.Wait(); err != nil {
		return
	}
	// Run scanners only if the user is entitled for Advanced Security
	if entitlements.Jas {
		results.ExtendedScanResults.EntitledForJas = entitlements.Jas
		results.JasError = runJasScannersAndSetResults(results.ExtendedScanResults, auditParams.DirectDependencies(), serverDetails, auditParams.workingDirs, auditParams.Progress(), auditParams.xrayGraphScanParams.MultiScanId)
	}
	return
}

func isEntitledForJas(xrayManager services.SecurityServiceManager, xrayVersion string) (entitled bool, err error) {
	if e := clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, xrayutils.EntitlementsMinVersion); e != nil {
		log.Debug(e)
		return
	}
	entitled, err = xrayManager.IsEntitled(xrayutils.ApplicabilityFeatureId)
	return
}

// checkEntitlements validates the entitlements for JAS and XSC.
func checkEntitlements(serverDetails *config.ServerDetails, auditParams *AuditParams) (entitlements *XrayEntitlements, err error) {
	var xrayManager services.SecurityServiceManager
	if xrayManager, auditParams.xrayVersion, err = xrayutils.CreateXrayServiceManagerAndGetVersion(serverDetails); err != nil {
		return
	}
	// Check entitlements
	var jasEntitle, xscEntitled bool
	if jasEntitle, err = isEntitledForJas(xrayManager, auditParams.xrayVersion); err != nil {
		return
	}
	// Setting serverDetails.XscVersion is important as this is how we determined if XSC is enabled or not.
	if xscEntitled, serverDetails.XscVersion, err = xrayManager.IsXscEnabled(); err != nil {
		return
	}
	entitlements = &XrayEntitlements{Jas: jasEntitle, Xsc: xscEntitled, errGroup: new(errgroup.Group)}
	log.Debug(fmt.Sprintf("entitlements results: JAS: %t XSC: %t", jasEntitle, xscEntitled))

	// Handle actions needed in case of specific entitlement.
	if entitlements.Jas {
		// Download the analyzer manager in a background routine.
		entitlements.errGroup.Go(rtutils.DownloadAnalyzerManagerIfNeeded)
	}
	if entitlements.Xsc {
		log.Info("XSC version:", serverDetails.XscVersion)
		auditParams.xscVersion = serverDetails.XscVersion
	}
	return
}
