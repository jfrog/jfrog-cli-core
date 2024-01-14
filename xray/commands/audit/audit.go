package audit

import (
	"errors"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/xray/scangraph"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/sync/errgroup"

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
		RepoPath: auditCmd.targetRepoPath,
		Watches:  auditCmd.watches,
		ScanType: services.Dependency,
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
		SetGraphBasicParams(auditCmd.AuditBasicParams).
		SetThirdPartyApplicabilityScan(auditCmd.thirdPartyApplicabilityScan).
		SetExclusions(auditCmd.exclusions)
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
	printScanResults := !(auditResults.ScaError != nil && !auditResults.IsScaIssuesFound())
	if printScanResults {
		if err = xrayutils.NewResultsWriter(auditResults).
			SetIsMultipleRootProject(auditResults.IsMultipleProject()).
			SetIncludeVulnerabilities(auditCmd.IncludeVulnerabilities).
			SetIncludeLicenses(auditCmd.IncludeLicenses).
			SetOutputFormat(auditCmd.OutputFormat()).
			SetPrintExtendedTable(auditCmd.PrintExtendedTable).
			SetExtraMessages(messages).
			SetScanType(services.Dependency).
			PrintScanResults(); err != nil {
			return
		}
	}
	if err = errors.Join(auditResults.ScaError, auditResults.JasError); err != nil {
		return
	}

	// Only in case Xray's context was given (!auditCmd.IncludeVulnerabilities), and the user asked to fail the build accordingly, do so.
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities && xrayutils.CheckIfFailBuild(auditResults.GetScaScansXrayResults()) {
		err = xrayutils.NewFailBuildError()
	}
	return
}

func (auditCmd *AuditCommand) CommandName() string {
	return "generic_audit"
}

// Runs an audit scan based on the provided auditParams.
// Returns an audit Results object containing all the scan results.
// If the current server is entitled for JAS, the advanced security results will be included in the scan results.
func RunAudit(auditParams *AuditParams) (results *xrayutils.Results, err error) {
	// Initialize Results struct
	results = xrayutils.NewAuditResults()

	serverDetails, err := auditParams.ServerDetails()
	if err != nil {
		return
	}
	var xrayManager *xray.XrayServicesManager
	if xrayManager, auditParams.xrayVersion, err = xrayutils.CreateXrayServiceManagerAndGetVersion(serverDetails); err != nil {
		return
	}
	if err = clientutils.ValidateMinimumVersion(clientutils.Xray, auditParams.xrayVersion, scangraph.GraphScanMinXrayVersion); err != nil {
		return
	}
	results.XrayVersion = auditParams.xrayVersion
	results.ExtendedScanResults.EntitledForJas, err = xrayutils.IsEntitledForJas(xrayManager, auditParams.xrayVersion)
	if err != nil {
		return
	}

	errGroup := new(errgroup.Group)
	if results.ExtendedScanResults.EntitledForJas {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(dependencies.DownloadAnalyzerManagerIfNeeded)
	}

	// The sca scan doesn't require the analyzer manager, so it can run separately from the analyzer manager download routine.
	results.ScaError = runScaScan(auditParams, results) // runScaScan(auditParams, results)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		err = errors.New("failed while trying to get Analyzer Manager: " + err.Error())
	}

	// Run scanners only if the user is entitled for Advanced Security
	if results.ExtendedScanResults.EntitledForJas {
		results.JasError = runJasScannersAndSetResults(results, auditParams.DirectDependencies(), serverDetails, auditParams.workingDirs, auditParams.Progress(), auditParams.xrayGraphScanParams.MultiScanId, auditParams.thirdPartyApplicabilityScan)
	}
	return
}
