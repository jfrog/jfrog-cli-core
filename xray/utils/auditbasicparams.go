package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
)

type AuditBasicParams struct {
	serverDetails           *config.ServerDetails
	outputFormat            OutputFormat
	progress                ioUtils.ProgressMgr
	directDependencies      []string
	excludeTestDependencies bool
	useWrapper              bool
	insecureTls             bool
	pipRequirementsFile     string
	technologies            []string
	args                    []string
	depsRepo                string
	ignoreConfigFile        bool
}

func (abp *AuditBasicParams) DirectDependencies() []string {
	return abp.directDependencies
}

func (abp *AuditBasicParams) AppendDirectDependencies(directDependencies []string) *AuditBasicParams {
	abp.directDependencies = append(abp.directDependencies, directDependencies...)
	return abp
}

func (abp *AuditBasicParams) ServerDetails() (*config.ServerDetails, error) {
	return abp.serverDetails, nil
}

func (abp *AuditBasicParams) SetServerDetails(serverDetails *config.ServerDetails) *AuditBasicParams {
	abp.serverDetails = serverDetails
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

func (abp *AuditBasicParams) SetNpmScope(depType string) *AuditBasicParams {
	switch depType {
	case "devOnly":
		abp.args = []string{"--dev"}
	case "prodOnly":
		abp.args = []string{"--prod"}
	}
	return abp
}

func (abp *AuditBasicParams) OutputFormat() OutputFormat {
	return abp.outputFormat
}

func (abp *AuditBasicParams) SetOutputFormat(format OutputFormat) *AuditBasicParams {
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
