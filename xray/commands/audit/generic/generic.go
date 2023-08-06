package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type GenericAuditCommand struct {
	watches                []string
	projectKey             string
	targetRepoPath         string
	IncludeVulnerabilities bool
	IncludeLicenses        bool
	Fail                   bool
	PrintExtendedTable     bool
	Params
}

func NewGenericAuditCommand() *GenericAuditCommand {
	return &GenericAuditCommand{Params: *NewAuditParams()}
}

func (auditCmd *GenericAuditCommand) SetWatches(watches []string) *GenericAuditCommand {
	auditCmd.watches = watches
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
		SetFixableOnly(auditCmd.fixableOnly).
		SetGraphBasicParams(auditCmd.GraphBasicParams)
	auditResults, err := RunAudit(auditParams)
	if err != nil {
		return err
	}
	if auditCmd.Progress() != nil {
		if err = auditCmd.Progress().Quit(); err != nil {
			return
		}
	}
	var messages []string
	if !auditResults.ExtendedScanResults.EntitledForJas {
		messages = []string{coreutils.PrintTitle("The ‘jf audit’ command also supports the ‘Contextual Analysis’ feature, which is included as part of the ‘Advanced Security’ package. This package isn't enabled on your system. Read more - ") + coreutils.PrintLink("https://jfrog.com/xray/")}
	}
	// Print Scan results on all cases except if errors accrued on Generic Audit command and no security/license issues found.
	printScanResults := !(auditResults.AuditError != nil && xrutils.IsEmptyScanResponse(auditResults.ExtendedScanResults.XrayResults))
	if printScanResults {
		err = xrutils.PrintScanResults(auditResults.ExtendedScanResults,
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
	if auditResults.AuditError != nil {
		err = auditResults.AuditError
		return
	}

	// Only in case Xray's context was given (!auditCmd.IncludeVulnerabilities), and the user asked to fail the build accordingly, do so.
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities && xrutils.CheckIfFailBuild(auditResults.ExtendedScanResults.XrayResults) {
		err = xrutils.NewFailBuildError()
	}
	return
}

func (auditCmd *GenericAuditCommand) CommandName() string {
	return "generic_audit"
}
