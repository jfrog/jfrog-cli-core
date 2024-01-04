package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
)

type AuditParams interface {
	DirectDependencies() []string
	AppendDependenciesForApplicabilityScan(directDependencies []string) *AuditBasicParams
	ServerDetails() (*config.ServerDetails, error)
	SetServerDetails(serverDetails *config.ServerDetails) *AuditBasicParams
	PipRequirementsFile() string
	SetPipRequirementsFile(requirementsFile string) *AuditBasicParams
	ExcludeTestDependencies() bool
	SetExcludeTestDependencies(excludeTestDependencies bool) *AuditBasicParams
	UseWrapper() bool
	SetUseWrapper(useWrapper bool) *AuditBasicParams
	InsecureTls() bool
	SetInsecureTls(insecureTls bool) *AuditBasicParams
	Technologies() []string
	SetTechnologies(technologies []string) *AuditBasicParams
	Progress() ioUtils.ProgressMgr
	SetProgress(progress ioUtils.ProgressMgr)
	Args() []string
	InstallCommandName() string
	InstallCommandArgs() []string
	SetNpmScope(depType string) *AuditBasicParams
	OutputFormat() format.OutputFormat
	DepsRepo() string
	SetDepsRepo(depsRepo string) *AuditBasicParams
	IgnoreConfigFile() bool
	SetIgnoreConfigFile(ignoreConfigFile bool) *AuditBasicParams
	IsMavenDepTreeInstalled() bool
	SetIsMavenDepTreeInstalled(isMavenDepTreeInstalled bool) *AuditBasicParams
}

type AuditBasicParams struct {
	serverDetails                    *config.ServerDetails
	outputFormat                     format.OutputFormat
	progress                         ioUtils.ProgressMgr
	excludeTestDependencies          bool
	useWrapper                       bool
	insecureTls                      bool
	ignoreConfigFile                 bool
	isMavenDepTreeInstalled          bool
	pipRequirementsFile              string
	depsRepo                         string
	installCommandName               string
	technologies                     []string
	args                             []string
	installCommandArgs               []string
	dependenciesForApplicabilityScan []string
}

func (abp *AuditBasicParams) DirectDependencies() []string {
	return abp.dependenciesForApplicabilityScan
}

func (abp *AuditBasicParams) AppendDependenciesForApplicabilityScan(directDependencies []string) *AuditBasicParams {
	abp.dependenciesForApplicabilityScan = append(abp.dependenciesForApplicabilityScan, directDependencies...)
	return abp
}

func (abp *AuditBasicParams) ServerDetails() (*config.ServerDetails, error) {
	return abp.serverDetails, nil
}

func (abp *AuditBasicParams) SetServerDetails(serverDetails *config.ServerDetails) *AuditBasicParams {
	abp.serverDetails = serverDetails
	return abp
}

func (abp *AuditBasicParams) SetInstallCommandArgs(installCommandArgs []string) *AuditBasicParams {
	abp.installCommandArgs = installCommandArgs
	return abp
}

func (abp *AuditBasicParams) SetInstallCommandName(installCommandName string) *AuditBasicParams {
	abp.installCommandName = installCommandName
	return abp
}

func (abp *AuditBasicParams) PipRequirementsFile() string {
	return abp.pipRequirementsFile
}

func (abp *AuditBasicParams) SetPipRequirementsFile(requirementsFile string) *AuditBasicParams {
	abp.pipRequirementsFile = requirementsFile
	return abp
}

func (abp *AuditBasicParams) ExcludeTestDependencies() bool {
	return abp.excludeTestDependencies
}

func (abp *AuditBasicParams) SetExcludeTestDependencies(excludeTestDependencies bool) *AuditBasicParams {
	abp.excludeTestDependencies = excludeTestDependencies
	return abp
}

func (abp *AuditBasicParams) UseWrapper() bool {
	return abp.useWrapper
}

func (abp *AuditBasicParams) SetUseWrapper(useWrapper bool) *AuditBasicParams {
	abp.useWrapper = useWrapper
	return abp
}

func (abp *AuditBasicParams) InsecureTls() bool {
	return abp.insecureTls
}

func (abp *AuditBasicParams) SetInsecureTls(insecureTls bool) *AuditBasicParams {
	abp.insecureTls = insecureTls
	return abp
}

func (abp *AuditBasicParams) Technologies() []string {
	return abp.technologies
}

func (abp *AuditBasicParams) SetTechnologies(technologies []string) *AuditBasicParams {
	abp.technologies = technologies
	return abp
}

func (abp *AuditBasicParams) Progress() ioUtils.ProgressMgr {
	return abp.progress
}

func (abp *AuditBasicParams) SetProgress(progress ioUtils.ProgressMgr) {
	abp.progress = progress
}

func (abp *AuditBasicParams) Args() []string {
	return abp.args
}

func (abp *AuditBasicParams) InstallCommandName() string {
	return abp.installCommandName
}

func (abp *AuditBasicParams) InstallCommandArgs() []string {
	return abp.installCommandArgs
}

func (abp *AuditBasicParams) SetNpmScope(depType string) *AuditBasicParams {
	switch depType {
	case "devOnly":
		abp.args = []string{"--dev"}
	case "prodOnly":
		abp.args = []string{"--prod"}
	}
	return abp
}

func (abp *AuditBasicParams) OutputFormat() format.OutputFormat {
	return abp.outputFormat
}

func (abp *AuditBasicParams) SetOutputFormat(format format.OutputFormat) *AuditBasicParams {
	abp.outputFormat = format
	return abp
}

func (abp *AuditBasicParams) DepsRepo() string {
	return abp.depsRepo
}

func (abp *AuditBasicParams) SetDepsRepo(depsRepo string) *AuditBasicParams {
	abp.depsRepo = depsRepo
	return abp
}

func (abp *AuditBasicParams) IgnoreConfigFile() bool {
	return abp.ignoreConfigFile
}

func (abp *AuditBasicParams) SetIgnoreConfigFile(ignoreConfigFile bool) *AuditBasicParams {
	abp.ignoreConfigFile = ignoreConfigFile
	return abp
}

func (abp *AuditBasicParams) IsMavenDepTreeInstalled() bool {
	return abp.isMavenDepTreeInstalled
}

func (abp *AuditBasicParams) SetIsMavenDepTreeInstalled(isMavenDepTreeInstalled bool) *AuditBasicParams {
	abp.isMavenDepTreeInstalled = isMavenDepTreeInstalled
	return abp
}
