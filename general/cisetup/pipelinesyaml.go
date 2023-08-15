package cisetup

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"gopkg.in/yaml.v3"
	"strconv"
)

const addRunFilesCmd = "add_run_files /tmp/jfrog/. jfrog"

type JFrogPipelinesYamlGenerator struct {
	VcsIntName string
	RtIntName  string
	SetupData  *CiSetupData
}

func (yg *JFrogPipelinesYamlGenerator) Generate() (pipelineBytes []byte, pipelineName string, err error) {
	pipelineName = yg.createPipelineName()
	gitResourceName := yg.createGitResourceName()
	biResourceName := yg.createBuildInfoResourceName()
	gitResource := yg.createGitResource(gitResourceName)
	biResource := yg.createBuildInfoResource(biResourceName)
	pipeline, err := yg.createPipeline(pipelineName, gitResourceName, biResourceName)
	if err != nil {
		return nil, "", err
	}
	pipelineYaml := PipelineYml{
		Resources: []Resource{gitResource, biResource},
		Pipelines: []Pipeline{pipeline},
	}
	pipelineBytes, err = yaml.Marshal(&pipelineYaml)
	return pipelineBytes, pipelineName, errorutils.CheckError(err)
}

func (yg *JFrogPipelinesYamlGenerator) getNpmBashCommands(serverId, gitResourceName, convertedBuildCmd string) []string {
	var commandsArray []string
	commandsArray = append(commandsArray, getCdToResourceCmd(gitResourceName))
	commandsArray = append(commandsArray, getJfrogCliConfigCmd(yg.RtIntName, serverId, true))
	commandsArray = append(commandsArray, getBuildToolConfigCmd(npmConfigCmdName, serverId, yg.SetupData.BuiltTechnology.VirtualRepo))
	commandsArray = append(commandsArray, convertedBuildCmd)
	commandsArray = append(commandsArray, jfrogCliBag)
	commandsArray = append(commandsArray, jfrogCliBce)
	return commandsArray
}

// Converts build tools commands to run via JFrog CLI.
func (yg *JFrogPipelinesYamlGenerator) convertNpmBuildCmd() (string, error) {
	// Replace npm-i.
	converted, err := replaceCmdWithRegexp(yg.SetupData.BuiltTechnology.BuildCmd, npmInstallRegexp, npmInstallRegexpReplacement)
	if err != nil {
		return "", err
	}
	// Replace npm-ci.
	return replaceCmdWithRegexp(converted, npmCiRegexp, npmCiRegexpReplacement)
}

func replaceCmdWithRegexp(buildCmd, cmdRegexp, replacement string) (string, error) {
	regexp, err := utils.GetRegExp(cmdRegexp)
	if err != nil {
		return "", err
	}
	return regexp.ReplaceAllString(buildCmd, replacement), nil
}

func (yg *JFrogPipelinesYamlGenerator) getPipelineEnvVars() map[string]string {
	return map[string]string{
		coreutils.CI:      strconv.FormatBool(true),
		buildNameEnvVar:   yg.SetupData.BuildName,
		buildNumberEnvVar: runNumberEnvVar,
		buildUrlEnvVar:    stepUrlEnvVar,
	}
}

func (yg *JFrogPipelinesYamlGenerator) createGitResource(gitResourceName string) Resource {
	return Resource{
		Name:         gitResourceName,
		ResourceType: GitRepo,
		ResourceConfiguration: GitRepoResourceConfiguration{
			Path:        yg.SetupData.GetRepoFullName(),
			GitProvider: yg.VcsIntName,
			BuildOn: BuildOn{
				PullRequestCreate: true,
			},
			Branches: IncludeExclude{Include: yg.SetupData.GitBranch},
		},
	}
}

func (yg *JFrogPipelinesYamlGenerator) createBuildInfoResource(buildInfoResourceName string) Resource {
	return Resource{
		Name:         buildInfoResourceName,
		ResourceType: BuildInfo,
		ResourceConfiguration: BuildInfoResourceConfiguration{
			SourceArtifactoryIntegration: yg.RtIntName,
			BuildName:                    yg.SetupData.BuildName,
			BuildNumber:                  runNumberEnvVar,
		},
	}
}

func (yg *JFrogPipelinesYamlGenerator) createPipeline(pipelineName, gitResourceName, buildInfoResourceName string) (Pipeline, error) {
	steps, err := yg.createSteps(gitResourceName, buildInfoResourceName)
	if err != nil {
		return Pipeline{}, err
	}
	return Pipeline{
		Name:  pipelineName,
		Steps: steps,
		Configuration: PipelineConfiguration{
			PipelineEnvVars: PipelineEnvVars{
				ReadOnlyEnvVars: yg.getPipelineEnvVars(),
			},
		},
	}, nil
}

func (yg *JFrogPipelinesYamlGenerator) createSteps(gitResourceName, buildInfoResourceName string) (steps []PipelineStep, err error) {
	var step PipelineStep

	switch yg.SetupData.BuiltTechnology.Type {
	case coreutils.Maven:
		step = yg.createMavenStep(gitResourceName)
	case coreutils.Gradle:
		step = yg.createGradleStep(gitResourceName)
	case coreutils.Npm:
		step, err = yg.createNpmStep(gitResourceName)
		if err != nil {
			return nil, err
		}
	}

	return []PipelineStep{step, yg.createBuildInfoStep(gitResourceName, step.Name, buildInfoResourceName)}, nil
}

func (yg *JFrogPipelinesYamlGenerator) createMavenStep(gitResourceName string) PipelineStep {
	return PipelineStep{
		Name:     createTechStepName(MvnBuild),
		StepType: MvnBuild,
		Configuration: &MavenStepConfiguration{
			NativeStepConfiguration: yg.getDefaultNativeStepConfiguration(gitResourceName),
			MvnCommand:              yg.SetupData.GetBuildCmdForNativeStep(),
			ResolverSnapshotRepo:    yg.SetupData.BuiltTechnology.VirtualRepo,
			ResolverReleaseRepo:     yg.SetupData.BuiltTechnology.VirtualRepo,
		},
		Execution: StepExecution{
			OnFailure: yg.getOnFailureCommands(),
		},
	}
}

func (yg *JFrogPipelinesYamlGenerator) getDefaultNativeStepConfiguration(gitResourceName string) NativeStepConfiguration {
	step := NativeStepConfiguration{
		BaseStepConfiguration: BaseStepConfiguration{
			EnvironmentVariables: map[string]string{
				buildStatusEnvVar: passResult,
			},
			Integrations: []StepIntegration{
				{
					Name: yg.RtIntName,
				},
			},
			InputResources: []StepResource{
				{
					Name: gitResourceName,
				},
			},
		},
		AutoPublishBuildInfo: false,
		ForceXrayScan:        false,
	}
	return step
}

func (yg *JFrogPipelinesYamlGenerator) createGradleStep(gitResourceName string) PipelineStep {
	return PipelineStep{
		Name:     createTechStepName(GradleBuild),
		StepType: GradleBuild,
		Configuration: &GradleStepConfiguration{
			NativeStepConfiguration: yg.getDefaultNativeStepConfiguration(gitResourceName),
			GradleCommand:           yg.SetupData.GetBuildCmdForNativeStep(),
			ResolverRepo:            yg.SetupData.BuiltTechnology.VirtualRepo,
		},
		Execution: StepExecution{
			OnFailure: yg.getOnFailureCommands(),
		},
	}
}

func (yg *JFrogPipelinesYamlGenerator) createNpmStep(gitResourceName string) (PipelineStep, error) {
	serverId := yg.createServerIdName()

	converted, err := yg.convertNpmBuildCmd()
	if err != nil {
		return PipelineStep{}, err
	}

	commands := yg.getNpmBashCommands(serverId, gitResourceName, converted)

	step := PipelineStep{
		Name:     createTechStepName(NpmBuild),
		StepType: Bash,
		Configuration: &BaseStepConfiguration{
			EnvironmentVariables: map[string]string{
				buildStatusEnvVar: passResult,
			},
			InputResources: []StepResource{
				{
					Name: gitResourceName,
				},
			},
			Integrations: []StepIntegration{
				{
					Name: yg.RtIntName,
				},
			},
		},
		Execution: StepExecution{
			OnExecute:  commands,
			OnComplete: []string{addRunFilesCmd},
			OnFailure:  yg.getOnFailureCommands(),
		},
	}
	return step, nil
}

func (yg *JFrogPipelinesYamlGenerator) createBuildInfoStep(gitResourceName, previousStepName, buildInfoResourceName string) PipelineStep {
	return PipelineStep{
		Name:     createTechStepName(PublishBuildInfo),
		StepType: PublishBuildInfo,
		Configuration: &NativeStepConfiguration{
			BaseStepConfiguration: BaseStepConfiguration{
				InputSteps: []InputStep{
					{
						Name: previousStepName,
					},
				},
				InputResources: []StepResource{
					{
						Name: gitResourceName,
					},
				},
				OutputResources: []StepResource{
					{
						Name: buildInfoResourceName,
					},
				},
			},
			ForceXrayScan: true,
		},
		Execution: StepExecution{
			OnComplete: []string{yg.getUpdateCommitStatusCmd(gitResourceName)},
		},
	}
}

type PipelineYml struct {
	Resources []Resource `yaml:"resources,omitempty"`
	Pipelines []Pipeline `yaml:"pipelines,omitempty"`
}

type ResourceType string

const (
	GitRepo   ResourceType = "GitRepo"
	BuildInfo ResourceType = "BuildInfo"
)

type Resource struct {
	Name                  string `yaml:"name,omitempty"`
	ResourceType          `yaml:"type,omitempty"`
	ResourceConfiguration `yaml:"configuration,omitempty"`
}

type ResourceConfiguration interface {
	ResourceConfigurationMarkerFunction()
}

type GitRepoResourceConfiguration struct {
	Path        string `yaml:"path,omitempty"`
	GitProvider string `yaml:"gitProvider,omitempty"`
	BuildOn     `yaml:"buildOn,omitempty"`
	Branches    IncludeExclude `yaml:"branches,omitempty"`
}

func (g GitRepoResourceConfiguration) ResourceConfigurationMarkerFunction() {}

type BuildInfoResourceConfiguration struct {
	SourceArtifactoryIntegration string      `yaml:"sourceArtifactory,omitempty"`
	BuildName                    string      `yaml:"buildName,omitempty"`
	BuildNumber                  json.Number `yaml:"buildNumber,omitempty"`
}

func (b BuildInfoResourceConfiguration) ResourceConfigurationMarkerFunction() {}

type IncludeExclude struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

type BuildOn struct {
	PullRequestCreate bool `yaml:"pullRequestCreate,omitempty"`
	Commit            bool `yaml:"commit,omitempty"`
}

type Pipeline struct {
	Name          string                `yaml:"name,omitempty"`
	Configuration PipelineConfiguration `yaml:"configuration,omitempty"`
	Steps         []PipelineStep        `yaml:"steps,omitempty"`
}

type PipelineConfiguration struct {
	Runtime         `yaml:"runtime,omitempty"`
	PipelineEnvVars `yaml:"environmentVariables,omitempty"`
}

type PipelineEnvVars struct {
	ReadOnlyEnvVars map[string]string `yaml:"readOnly,omitempty"`
}

type RuntimeType string

const (
	Image RuntimeType = "image"
)

type Runtime struct {
	RuntimeType `yaml:"type,omitempty"`
	Image       RuntimeImage `yaml:"image,omitempty"`
}

type RuntimeImage struct {
	Custom CustomImage `yaml:"custom,omitempty"`
}

type CustomImage struct {
	Name             string `yaml:"name,omitempty"`
	Tag              string `yaml:"tag,omitempty"`
	Options          string `yaml:"options,omitempty"`
	Registry         string `yaml:"registry,omitempty"`
	SourceRepository string `yaml:"sourceRepository,omitempty"`
	Region           string `yaml:"region,omitempty"`
}

type StepType string

const (
	MvnBuild         StepType = "MvnBuild"
	GradleBuild      StepType = "GradleBuild"
	NpmBuild         StepType = "NpmBuild"
	Bash             StepType = "Bash"
	PublishBuildInfo StepType = "PublishBuildInfo"
)

type PipelineStep struct {
	Name          string `yaml:"name,omitempty"`
	StepType      `yaml:"type,omitempty"`
	Configuration StepConfiguration `yaml:"configuration,omitempty"`
	Execution     StepExecution     `yaml:"execution,omitempty"`
}

type StepConfiguration interface {
	appendInputSteps([]InputStep)
}

type BaseStepConfiguration struct {
	EnvironmentVariables map[string]string `yaml:"environmentVariables,omitempty"`
	Integrations         []StepIntegration `yaml:"integrations,omitempty"`
	InputResources       []StepResource    `yaml:"inputResources,omitempty"`
	OutputResources      []StepResource    `yaml:"outputResources,omitempty"`
	InputSteps           []InputStep       `yaml:"inputSteps,omitempty"`
}

func (b *BaseStepConfiguration) appendInputSteps(steps []InputStep) {
	b.InputSteps = append(b.InputSteps, steps...)
}

type NativeStepConfiguration struct {
	BaseStepConfiguration `yaml:",inline"`
	ForceXrayScan         bool `yaml:"forceXrayScan,omitempty"`
	FailOnScan            bool `yaml:"failOnScan,omitempty"`
	AutoPublishBuildInfo  bool `yaml:"autoPublishBuildInfo,omitempty"`
}

type MavenStepConfiguration struct {
	NativeStepConfiguration `yaml:",inline"`
	MvnCommand              string `yaml:"mvnCommand,omitempty"`
	ResolverSnapshotRepo    string `yaml:"resolverSnapshotRepo,omitempty"`
	ResolverReleaseRepo     string `yaml:"resolverReleaseRepo,omitempty"`
	DeployerSnapshotRepo    string `yaml:"deployerSnapshotRepo,omitempty"`
	DeployerReleaseRepo     string `yaml:"deployerReleaseRepo,omitempty"`
}

type GradleStepConfiguration struct {
	NativeStepConfiguration `yaml:",inline"`
	GradleCommand           string `yaml:"gradleCommand,omitempty"`
	ResolverRepo            string `yaml:"resolverRepo,omitempty"`
	UsesPlugin              bool   `yaml:"usesPlugin,omitempty"`
	UseWrapper              bool   `yaml:"useWrapper,omitempty"`
}

type StepIntegration struct {
	Name string `yaml:"name,omitempty"`
}

type StepResource struct {
	Name string `yaml:"name,omitempty"`
}

type InputStep struct {
	Name string `yaml:"name,omitempty"`
}

type StepExecution struct {
	OnStart    []string `yaml:"onStart,omitempty"`
	OnExecute  []string `yaml:"onExecute,omitempty"`
	OnComplete []string `yaml:"onComplete,omitempty"`
	OnSuccess  []string `yaml:"onSuccess,omitempty"`
	OnFailure  []string `yaml:"onFailure,omitempty"`
}

func (yg *JFrogPipelinesYamlGenerator) getOnFailureCommands() []string {
	return []string{getExportCmd(buildStatusEnvVar, failResult),
		jfrogCliBce,
		jfrogCliBp}
}

func (yg *JFrogPipelinesYamlGenerator) getUpdateCommitStatusCmd(gitResourceName string) string {
	return updateCommitStatusCmd + " " + gitResourceName
}

func (yg *JFrogPipelinesYamlGenerator) createGitResourceName() string {
	return createPipelinesSuitableName(yg.SetupData, "gitResource")
}

func (yg *JFrogPipelinesYamlGenerator) createBuildInfoResourceName() string {
	return createPipelinesSuitableName(yg.SetupData, "buildInfoResource")
}

func (yg *JFrogPipelinesYamlGenerator) createPipelineName() string {
	return createPipelinesSuitableName(yg.SetupData, "pipeline")
}

func (yg *JFrogPipelinesYamlGenerator) createServerIdName() string {
	return createPipelinesSuitableName(yg.SetupData, "serverId")
}

func createTechStepName(stepType StepType) string {
	return string(stepType) + "Step"
}
