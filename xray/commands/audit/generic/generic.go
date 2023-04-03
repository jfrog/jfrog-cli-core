package audit

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic/jas"
	"os"

	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type GenericAuditCommand struct {
	serverDetails           *config.ServerDetails
	OutputFormat            xrutils.OutputFormat
	watches                 []string
	workingDirs             []string
	projectKey              string
	targetRepoPath          string
	IncludeVulnerabilities  bool
	IncludeLicenses         bool
	Fail                    bool
	PrintExtendedTable      bool
	excludeTestDependencies bool
	useWrapper              bool
	insecureTls             bool
	args                    []string
	technologies            []string
	requirementsFile        string
	progress                ioUtils.ProgressMgr
}

func NewGenericAuditCommand() *GenericAuditCommand {
	return &GenericAuditCommand{}
}

func (auditCmd *GenericAuditCommand) SetServerDetails(server *config.ServerDetails) *GenericAuditCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetOutputFormat(format xrutils.OutputFormat) *GenericAuditCommand {
	auditCmd.OutputFormat = format
	return auditCmd
}

func (auditCmd *GenericAuditCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
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

func (auditCmd *GenericAuditCommand) CreateXrayGraphScanParams() services.XrayGraphScanParams {
	params := services.XrayGraphScanParams{
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
	server, err := auditCmd.ServerDetails()
	if err != nil {
		return
	}
	auditParams := NewAuditParams().
		SetXrayGraphScanParams(auditCmd.CreateXrayGraphScanParams()).
		SetServerDetails(server).
		SetExcludeTestDeps(auditCmd.excludeTestDependencies).
		SetUseWrapper(auditCmd.useWrapper).
		SetInsecureTLS(auditCmd.insecureTls).
		SetArgs(auditCmd.args).
		SetProgressBar(auditCmd.progress).
		SetRequirementsFile(auditCmd.requirementsFile).
		SetWorkingDirs(auditCmd.workingDirs).
		SetTechnologies(auditCmd.technologies...)
	results, isMultipleRootProject, auditErr := GenericAudit(auditParams)

	applicabilityScanResults, err := getApplicabilityScanResults(results) // todo pass to print function
	fmt.Println(applicabilityScanResults)
	if err != nil {
		return err
	}

	if auditCmd.progress != nil {
		err = auditCmd.progress.Quit()
		if err != nil {
			return
		}
	}
	// Print Scan results on all cases except if errors accrued on Generic Audit command and no security/license issues found.
	printScanResults := !(auditErr != nil && xrutils.IsEmptyScanResponse(results))
	if printScanResults {
		err = xrutils.PrintScanResults(results,
			nil,
			auditCmd.OutputFormat,
			auditCmd.IncludeVulnerabilities,
			auditCmd.IncludeLicenses,
			isMultipleRootProject,
			auditCmd.PrintExtendedTable,
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

func getApplicabilityScanResults(results []services.ScanResponse) ([]string, error) {
	applicabilityScanManager := jas.NewApplicabilityScanManager(results)
	err := applicabilityScanManager.Run()
	if err != nil {
		// todo log error, continue
		deleteFileError := applicabilityScanManager.DeleteApplicabilityScanProcessFiles()
		if deleteFileError != nil {
			// todo log error, stop execution
			return nil, deleteFileError
		}
	}
	applicabilityScanResults := applicabilityScanManager.GetApplicableVulnerabilities()
	return applicabilityScanResults, nil
}

func (auditCmd *GenericAuditCommand) CommandName() string {
	return "generic_audit"
}

func (auditCmd *GenericAuditCommand) SetNpmScope(depType string) *GenericAuditCommand {
	switch depType {
	case "devOnly":
		auditCmd.args = []string{"--dev"}
	case "prodOnly":
		auditCmd.args = []string{"--prod"}
	}
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetPipRequirementsFile(requirementsFile string) *GenericAuditCommand {
	auditCmd.requirementsFile = requirementsFile
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetExcludeTestDependencies(excludeTestDependencies bool) *GenericAuditCommand {
	auditCmd.excludeTestDependencies = excludeTestDependencies
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetUseWrapper(useWrapper bool) *GenericAuditCommand {
	auditCmd.useWrapper = useWrapper
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetInsecureTls(insecureTls bool) *GenericAuditCommand {
	auditCmd.insecureTls = insecureTls
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetTechnologies(technologies []string) *GenericAuditCommand {
	auditCmd.technologies = technologies
	return auditCmd
}

func (auditCmd *GenericAuditCommand) SetProgress(progress ioUtils.ProgressMgr) {
	auditCmd.progress = progress
}
