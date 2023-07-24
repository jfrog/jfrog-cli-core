package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

type GraphBasicParams struct {
	serverDetails           *config.ServerDetails
	outputFormat            OutputFormat
	progress                ioUtils.ProgressMgr
	fullDependenciesTree    []*xrayUtils.GraphNode
	excludeTestDependencies bool
	useWrapper              bool
	insecureTls             bool
	pipRequirementsFile     string
	technologies            []string
	args                    []string
	depsRepo                string
	ignoreConfigFile        bool
}

func (gbp *GraphBasicParams) FullDependenciesTree() []*xrayUtils.GraphNode {
	return gbp.fullDependenciesTree
}

func (gbp *GraphBasicParams) SetFullDependenciesTree(fullDependenciesTree []*xrayUtils.GraphNode) *GraphBasicParams {
	gbp.fullDependenciesTree = fullDependenciesTree
	return gbp
}

func (gbp *GraphBasicParams) ServerDetails() (*config.ServerDetails, error) {
	return gbp.serverDetails, nil
}

func (gbp *GraphBasicParams) SetServerDetails(serverDetails *config.ServerDetails) *GraphBasicParams {
	gbp.serverDetails = serverDetails
	return gbp
}

func (gbp *GraphBasicParams) PipRequirementsFile() string {
	return gbp.pipRequirementsFile
}

func (gbp *GraphBasicParams) SetPipRequirementsFile(requirementsFile string) *GraphBasicParams {
	gbp.pipRequirementsFile = requirementsFile
	return gbp
}

func (gbp *GraphBasicParams) ExcludeTestDependencies() bool {
	return gbp.excludeTestDependencies
}

func (gbp *GraphBasicParams) SetExcludeTestDependencies(excludeTestDependencies bool) *GraphBasicParams {
	gbp.excludeTestDependencies = excludeTestDependencies
	return gbp
}

func (gbp *GraphBasicParams) UseWrapper() bool {
	return gbp.useWrapper
}

func (gbp *GraphBasicParams) SetUseWrapper(useWrapper bool) *GraphBasicParams {
	gbp.useWrapper = useWrapper
	return gbp
}

func (gbp *GraphBasicParams) InsecureTls() bool {
	return gbp.insecureTls
}

func (gbp *GraphBasicParams) SetInsecureTls(insecureTls bool) *GraphBasicParams {
	gbp.insecureTls = insecureTls
	return gbp
}

func (gbp *GraphBasicParams) Technologies() []string {
	return gbp.technologies
}

func (gbp *GraphBasicParams) SetTechnologies(technologies []string) *GraphBasicParams {
	gbp.technologies = technologies
	return gbp
}

func (gbp *GraphBasicParams) Progress() ioUtils.ProgressMgr {
	return gbp.progress
}

func (gbp *GraphBasicParams) SetProgress(progress ioUtils.ProgressMgr) {
	gbp.progress = progress
}

func (gbp *GraphBasicParams) Args() []string {
	return gbp.args
}

// Adds arguments to exclude development dependencies during scanning.
func (gbp *GraphBasicParams) SetExcludeDevDependencies(technology coreutils.Technology) *GraphBasicParams {
	if technology == coreutils.Npm {
		gbp.args = []string{"--dev"}
	}
	return gbp
}

func (gbp *GraphBasicParams) OutputFormat() OutputFormat {
	return gbp.outputFormat
}

func (gbp *GraphBasicParams) SetOutputFormat(format OutputFormat) *GraphBasicParams {
	gbp.outputFormat = format
	return gbp
}

func (gbp *GraphBasicParams) DepsRepo() string {
	return gbp.depsRepo
}

func (gbp *GraphBasicParams) SetDepsRepo(depsRepo string) *GraphBasicParams {
	gbp.depsRepo = depsRepo
	return gbp
}

func (gbp *GraphBasicParams) IgnoreConfigFile() bool {
	return gbp.ignoreConfigFile
}

func (gbp *GraphBasicParams) SetIgnoreConfigFile(ignoreConfigFile bool) *GraphBasicParams {
	gbp.ignoreConfigFile = ignoreConfigFile
	return gbp
}
