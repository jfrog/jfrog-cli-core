package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/jas"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/sync/errgroup"
)

type GenericAuditCommand struct {
	watches                []string
	workingDirs            []string
	projectKey             string
	targetRepoPath         string
	minSeverityFilter      string
	fixableOnly            bool
	IncludeVulnerabilities bool
	IncludeLicenses        bool
	Fail                   bool
	PrintExtendedTable     bool
	*xrutils.GraphBasicParams
}

func NewGenericAuditCommand() *GenericAuditCommand {
	return &GenericAuditCommand{GraphBasicParams: &xrutils.GraphBasicParams{}}
}

func (auditCmd *GenericAuditCommand) SetWatches(watches []string) *GenericAuditCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetWorkingDirs(dirs []string) *GenericAuditCommand {
	auditCmd.workingDirs = dirs
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetProject(project string) *GenericAuditCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetTargetRepoPath(repoPath string) *GenericAuditCommand {
	auditCmd.targetRepoPath = repoPath
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetIncludeVulnerabilities(include bool) *GenericAuditCommand {
	auditCmd.IncludeVulnerabilities = include
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetIncludeLicenses(include bool) *GenericAuditCommand {
	auditCmd.IncludeLicenses = include
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetFail(fail bool) *GenericAuditCommand {
	auditCmd.Fail = fail
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetPrintExtendedTable(printExtendedTable bool) *GenericAuditCommand {
	auditCmd.PrintExtendedTable = printExtendedTable
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetMinSeverityFilter(minSeverityFilter string) *GenericAuditCommand {
	auditCmd.minSeverityFilter = minSeverityFilter
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetFixableOnly(fixable bool) *GenericAuditCommand {
	auditCmd.fixableOnly = fixable
	return auditCmd
}

func (auditCmd *GenericAuditCommand) CreateXrayGraphScanParams() *services.XrayGraphScanParams {
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

func (auditCmd *GenericAuditCommand) Run() (err error) {
	auditParams := NewAuditParams().
		SetXrayGraphScanParams(auditCmd.CreateXrayGraphScanParams()).
		SetWorkingDirs(auditCmd.workingDirs).
		SetMinSeverityFilter(auditCmd.minSeverityFilter).
		SetFixableOnly(auditCmd.fixableOnly)
	auditParams.GraphBasicParams = auditCmd.GraphBasicParams

	serverDetails, err := auditParams.ServerDetails()
	if err != nil {
		return err
	}
	xrayManager, xrayVersion, err := commandsutils.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return err
	}
	auditParams.xrayVersion = xrayVersion
	var entitled bool
	errGroup := new(errgroup.Group)
	if err = coreutils.ValidateMinimumVersion(coreutils.Xray, xrayVersion, xrutils.EntitlementsMinVersion); err == nil {
		entitled, err = xrayManager.IsEntitled(xrutils.ApplicabilityFeatureId)
		if err != nil {
			return err
		}
	} else {
		entitled = false
		log.Debug("Entitlements check for ‘Advanced Security’ package failed:\n" + err.Error())
	}
	if entitled {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(utils.DownloadAnalyzerManagerIfNeeded)
	}
	results, isMultipleRootProject, auditErr := GenericAudit(auditParams)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		return err
	}
	extendedScanResults := &xrutils.ExtendedScanResults{XrayResults: results, ApplicabilityScanResults: nil, EntitledForJas: false}
	// Try to run contextual analysis only if the user is entitled for advance security
	if entitled {
		extendedScanResults, err = jas.GetExtendedScanResults(results, auditParams.FullDependenciesTree(), serverDetails)
		if err != nil {
			return err
		}
	}
	if auditCmd.Progress() != nil {
		if err = auditCmd.Progress().Quit(); err != nil {
			return
		}
	}
	var messages []string
	if !entitled {
		messages = []string{coreutils.PrintTitle("The ‘jf audit’ command also supports the ‘Contextual Analysis’ feature, which is included as part of the ‘Advanced Security’ package. This package isn't enabled on your system. Read more - ") + coreutils.PrintLink("https://jfrog.com/security-and-compliance")}
	}
	// Print Scan results on all cases except if errors accrued on Generic Audit command and no security/license issues found.
	printScanResults := !(auditErr != nil && xrutils.IsEmptyScanResponse(results))
	if printScanResults {
		err = xrutils.PrintScanResults(extendedScanResults,
			nil,
			auditCmd.OutputFormat(),
			auditCmd.IncludeVulnerabilities,
			auditCmd.IncludeLicenses,
			isMultipleRootProject,
			auditCmd.PrintExtendedTable, false, messages,
		)
		if err != nil {
			return
		}
	}
	if auditErr != nil {
		err = auditErr
		return
	}

	// Only in case Xray's context was given (!auditCmd.IncludeVulnerabilities) and the user asked to fail the build accordingly, do so.
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities && xrutils.CheckIfFailBuild(results) {
		err = xrutils.NewFailBuildError()
	}
	return
}

func (auditCmd *GenericAuditCommand) CommandName() string {
	return "generic_audit"
}
