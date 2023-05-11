package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

type GraphBasicParams struct {
	serverDetails           *config.ServerDetails
	OutputFormat            OutputFormat
	Progress                ioUtils.ProgressMgr
	DependencyTrees         []*xrayUtils.GraphNode
	ReleasesRepo            string
	ExcludeTestDependencies bool
	UseWrapper              bool
	InsecureTls             bool
	RequirementsFile        string
	Technologies            []string
	Args                    []string
	depsRepo                string
	IgnoreConfigFile        bool
}

func (gbp *GraphBasicParams) SetServerDetails(serverDetails *config.ServerDetails) *GraphBasicParams {
	gbp.serverDetails = serverDetails
	return gbp
}

func (gbp *GraphBasicParams) ServerDetails() (*config.ServerDetails, error) {
	return gbp.serverDetails, nil
}

func (gbp *GraphBasicParams) SetPipRequirementsFile(requirementsFile string) *GraphBasicParams {
	gbp.RequirementsFile = requirementsFile
	return gbp
}

func (gbp *GraphBasicParams) SetExcludeTestDependencies(excludeTestDependencies bool) *GraphBasicParams {
	gbp.ExcludeTestDependencies = excludeTestDependencies
	return gbp
}

func (gbp *GraphBasicParams) SetUseWrapper(useWrapper bool) *GraphBasicParams {
	gbp.UseWrapper = useWrapper
	return gbp
}

func (gbp *GraphBasicParams) SetInsecureTls(insecureTls bool) *GraphBasicParams {
	gbp.InsecureTls = insecureTls
	return gbp
}

func (gbp *GraphBasicParams) SetTechnologies(technologies []string) *GraphBasicParams {
	gbp.Technologies = technologies
	return gbp
}

func (gbp *GraphBasicParams) SetProgress(progress ioUtils.ProgressMgr) {
	gbp.Progress = progress
}

func (gbp *GraphBasicParams) SetNpmScope(depType string) *GraphBasicParams {
	switch depType {
	case "devOnly":
		gbp.Args = []string{"--dev"}
	case "prodOnly":
		gbp.Args = []string{"--prod"}
	}
	return gbp
}

func (gbp *GraphBasicParams) SetOutputFormat(format OutputFormat) *GraphBasicParams {
	gbp.OutputFormat = format
	return gbp
}

func (gbp *GraphBasicParams) SetDepsRepo(depsRepo string) *GraphBasicParams {
	gbp.depsRepo = depsRepo
	return gbp
}

func (gbp *GraphBasicParams) DepsRepo() string {
	return gbp.depsRepo
}

func (gbp *GraphBasicParams) SetIgnoreConfigFile(ignoreConfigFile bool) *GraphBasicParams {
	gbp.IgnoreConfigFile = ignoreConfigFile
	return gbp
}
