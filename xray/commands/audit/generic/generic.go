package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
)

type GenericAuditCommand struct {
	audit.AuditCommand
	excludeTestDependencies bool
	useWrapper              bool
	insecureTls             bool
	args                    []string
	technologies            []string
}

func NewGenericAuditCommand(auditCmd audit.AuditCommand) *GenericAuditCommand {
	return &GenericAuditCommand{AuditCommand: auditCmd}
}

func (auditCmd *GenericAuditCommand) Run() (err error) {
	server, err := auditCmd.ServerDetails()
	if err != nil {
		return err
	}
	results, isMultipleRootProject, err := GenericAudit(auditCmd.CreateXrayGraphScanParams(), server, auditCmd.excludeTestDependencies, auditCmd.useWrapper, auditCmd.insecureTls, auditCmd.args, auditCmd.technologies...)
	if err != nil {
		return err
	}

	err = xrutils.PrintScanResults(results, auditCmd.OutputFormat == xrutils.Table, auditCmd.IncludeVulnerabilities, auditCmd.IncludeLicenses, isMultipleRootProject, auditCmd.PrintExtendedTable)
	if err != nil {
		return err
	}
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities {
		if xrutils.CheckIfFailBuild(results) {
			return xrutils.NewFailBuildError()
		}
	}
	return nil
}

func (auditCmd *GenericAuditCommand) CommandName() string {
	return "generic_audit"
}

func (auditCmd *GenericAuditCommand) SetNpmArgs(depType string) *GenericAuditCommand {
	switch depType {
	case "devOnly":
		auditCmd.args = []string{"--dev"}
	case "prodOnly":
		auditCmd.args = []string{"--prod"}
	}
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
