package cisetup

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"gopkg.in/yaml.v2"
	"strconv"
	"strings"
)

const (
	jfrogCliFullImgName   = "releases-docker.jfrog.io/jfrog/jfrog-cli-full"
	jfrogCliFullImgTag    = "latest"
	m2pathCmd             = "MVN_PATH=`which mvn` && export M2_HOME=`readlink -f $MVN_PATH | xargs dirname | xargs dirname`"
	jfrogCliRtPrefix      = "jfrog rt"
	jfrogCliConfig        = "jfrog c add"
	jfrogCliBce           = "jfrog rt bce"
	jfrogCliBag           = "jfrog rt bag"
	jfrogCliBp            = "jfrog rt bp"
	buildNameEnvVar       = "JFROG_CLI_BUILD_NAME"
	buildNumberEnvVar     = "JFROG_CLI_BUILD_NUMBER"
	buildUrlEnvVar        = "JFROG_CLI_BUILD_URL"
	buildStatusEnvVar     = "JFROG_BUILD_STATUS"
	runNumberEnvVar       = "$run_number"
	stepUrlEnvVar         = "$step_url"
	updateCommitStatusCmd = "update_commit_status"

	gradleConfigCmdName  = "gradle-config"
	npmConfigCmdName     = "npm-config"
	mvnConfigCmdName     = "mvn-config"
	serverIdResolve      = "server-id-resolve"
	repoResolveReleases  = "repo-resolve-releases"
	repoResolveSnapshots = "repo-resolve-snapshots"
	repoResolve          = "repo-resolve"

	passResult = "PASS"
	failResult = "FAIL"

	urlFlag    = "url"
	rtUrlFlag  = "artifactory-url"
	userFlag   = "user"
	apikeyFlag = "apikey"

	// Replace exe (group 2) with "jfrog rt exe" while maintaining preceding (if any) and succeeding spaces.
	mvnGradleRegexp             = `(^|\s)(mvn|gradle)(\s)`
	mvnGradleRegexpReplacement  = `${1}jfrog rt ${2}${3}`
	npmInstallRegexp            = `(^|\s)(npm i|npm install)(\s|$)`
	npmInstallRegexpReplacement = `${1}jfrog rt npmi${3}`
	npmCiRegexp                 = `(^|\s)(npm ci)(\s|$)`
	npmCiRegexpReplacement      = `${1}jfrog rt npmci${3}`
)

type JFrogPipelinesYamlGenerator struct {
	VcsIntName string
	RtIntName  string
	SetupData  *CiSetupData
}

func (yg *JFrogPipelinesYamlGenerator) Generate() (pipelineBytes []byte, pipelineName string, err error) {
	pipelineName = yg.createPipelineName()
	gitResourceName := yg.createGitResourceName()
	serverId := yg.createServerIdName()

	converted, err := yg.convertBuildCmd()
	if err != nil {
		return nil, "", err
	}

	pipelinesCommands := yg.getPipelineCommands(serverId, gitResourceName, converted)
	gitResource := yg.createGitResource(gitResourceName)
	pipeline := yg.createPipeline(pipelineName, gitResourceName, pipelinesCommands)
	pipelineYaml := PipelineYml{
		Resources: []Resource{gitResource},
		Pipelines: []Pipeline{pipeline},
	}
	pipelineBytes, err = yaml.Marshal(&pipelineYaml)
	return pipelineBytes, pipelineName, errorutils.CheckError(err)
}

func (yg *JFrogPipelinesYamlGenerator) getPipelineCommands(serverId, gitResourceName, convertedBuildCmd string) []string {
	var commandsArray []string
	commandsArray = append(commandsArray, yg.getCdToResourceCmd(gitResourceName))
	commandsArray = append(commandsArray, yg.getExportsCommands(yg.SetupData)...)
	commandsArray = append(commandsArray, yg.getJfrogCliConfigCmd(yg.RtIntName, serverId))
	commandsArray = append(commandsArray, yg.getTechConfigsCommands(serverId, yg.SetupData)...)
	commandsArray = append(commandsArray, convertedBuildCmd)
	commandsArray = append(commandsArray, jfrogCliBag)
	commandsArray = append(commandsArray, jfrogCliBce)
	commandsArray = append(commandsArray, jfrogCliBp)
	return commandsArray
}

// Converts build tools commands to run via JFrog CLI.
func (yg *JFrogPipelinesYamlGenerator) convertBuildCmd() (string, error) {
	// Replace mvn, gradle.
	converted, err := replaceCmdWithRegexp(yg.SetupData.BuildCommand, mvnGradleRegexp, mvnGradleRegexpReplacement)
	if err != nil {
		return "", err
	}
	// Replace npm-i.
	converted, err = replaceCmdWithRegexp(converted, npmInstallRegexp, npmInstallRegexpReplacement)
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

func (yg *JFrogPipelinesYamlGenerator) getCdToResourceCmd(gitResourceName string) string {
	return fmt.Sprintf("cd $res_%s_resourcePath", gitResourceName)
}

func (yg *JFrogPipelinesYamlGenerator) getIntDetailForCmd(intName, detail string) string {
	return fmt.Sprintf("$int_%s_%s", intName, detail)
}

func (yg *JFrogPipelinesYamlGenerator) getFlagSyntax(flagName string) string {
	return fmt.Sprintf("--%s", flagName)
}

func (yg *JFrogPipelinesYamlGenerator) getJfrogCliConfigCmd(rtIntName, serverId string) string {
	return strings.Join([]string{
		jfrogCliConfig, serverId,
		yg.getFlagSyntax(rtUrlFlag), yg.getIntDetailForCmd(rtIntName, urlFlag),
		yg.getFlagSyntax(userFlag), yg.getIntDetailForCmd(rtIntName, userFlag),
		yg.getFlagSyntax(apikeyFlag), yg.getIntDetailForCmd(rtIntName, apikeyFlag),
		"--enc-password=false",
	}, " ")
}

func (yg *JFrogPipelinesYamlGenerator) getTechConfigsCommands(serverId string, data *CiSetupData) []string {
	var configs []string
	if used, ok := data.DetectedTechnologies[Maven]; ok && used {
		configs = append(configs, m2pathCmd)
		configs = append(configs, yg.getMavenConfigCmd(serverId, data.ArtifactoryVirtualRepos[Maven]))
	}
	if used, ok := data.DetectedTechnologies[Gradle]; ok && used {
		configs = append(configs, yg.getBuildToolConfigCmd(gradleConfigCmdName, serverId, data.ArtifactoryVirtualRepos[Gradle]))
	}
	if used, ok := data.DetectedTechnologies[Npm]; ok && used {
		configs = append(configs, yg.getBuildToolConfigCmd(npmConfigCmdName, serverId, data.ArtifactoryVirtualRepos[Npm]))
	}
	return configs
}

func (yg *JFrogPipelinesYamlGenerator) getMavenConfigCmd(serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, mvnConfigCmdName,
		yg.getFlagSyntax(serverIdResolve), serverId,
		yg.getFlagSyntax(repoResolveReleases), repo,
		yg.getFlagSyntax(repoResolveSnapshots), repo,
	}, " ")
}

func (yg *JFrogPipelinesYamlGenerator) getBuildToolConfigCmd(configCmd, serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, configCmd,
		yg.getFlagSyntax(serverIdResolve), serverId,
		yg.getFlagSyntax(repoResolve), repo,
	}, " ")
}

func (yg *JFrogPipelinesYamlGenerator) getExportsCommands(vcsData *CiSetupData) []string {
	return []string{
		yg.getExportCmd(coreutils.CI, strconv.FormatBool(true)),
		yg.getExportCmd(buildNameEnvVar, vcsData.BuildName),
		yg.getExportCmd(buildNumberEnvVar, runNumberEnvVar),
		yg.getExportCmd(buildUrlEnvVar, stepUrlEnvVar),
		yg.getExportCmd(buildStatusEnvVar, passResult),
	}
}

func (yg *JFrogPipelinesYamlGenerator) getExportCmd(key, value string) string {
	return fmt.Sprintf("export %s=%s", key, value)
}

func (yg *JFrogPipelinesYamlGenerator) createGitResource(gitResourceName string) Resource {
	return Resource{
		Name:         gitResourceName,
		ResourceType: GitRepo,
		ResourceConfiguration: ResourceConfiguration{
			Path:        yg.SetupData.GetRepoFullName(),
			GitProvider: yg.VcsIntName,
			BuildOn: BuildOn{
				PullRequestCreate: true,
			},
			Branches: IncludeExclude{Include: yg.SetupData.GitBranch},
		},
	}
}

func (yg *JFrogPipelinesYamlGenerator) createPipeline(pipelineName, gitResourceName string, commands []string) Pipeline {
	return Pipeline{
		Name: pipelineName,
		Configuration: PipelineConfiguration{
			Runtime{
				RuntimeType: Image,
				Image: RuntimeImage{
					Custom: CustomImage{
						Name: jfrogCliFullImgName,
						Tag:  jfrogCliFullImgTag,
					},
				},
			},
		},
		Steps: []PipelineStep{
			{
				Name:     "Build",
				StepType: "Bash",
				Configuration: StepConfiguration{
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
					OnComplete: []string{yg.getUpdateCommitStatusCmd(gitResourceName)},
					OnFailure:  yg.getOnFailureCommands(),
				},
			},
		},
	}
}

type PipelineYml struct {
	Resources []Resource `yaml:"resources,omitempty"`
	Pipelines []Pipeline `yaml:"pipelines,omitempty"`
}

type Resource struct {
	Name                  string `yaml:"name,omitempty"`
	ResourceType          `yaml:"type,omitempty"`
	ResourceConfiguration `yaml:"configuration,omitempty"`
}

type ResourceType string

const (
	GitRepo ResourceType = "GitRepo"
)

type ResourceConfiguration struct {
	Path        string `yaml:"path,omitempty"`
	GitProvider string `yaml:"gitProvider,omitempty"`
	BuildOn     `yaml:"buildOn,omitempty"`
	Branches    IncludeExclude `yaml:"branches,omitempty"`
}

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
	Runtime `yaml:"runtime,omitempty"`
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

type PipelineStep struct {
	Name          string            `yaml:"name,omitempty"`
	StepType      string            `yaml:"type,omitempty"`
	Configuration StepConfiguration `yaml:"configuration,omitempty"`
	Execution     StepExecution     `yaml:"execution,omitempty"`
}

type StepConfiguration struct {
	InputResources []StepResource    `yaml:"inputResources,omitempty"`
	Integrations   []StepIntegration `yaml:"integrations,omitempty"`
}

type StepResource struct {
	Name string `yaml:"name,omitempty"`
}

type StepIntegration struct {
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
	return []string{yg.getExportCmd(buildStatusEnvVar, failResult),
		jfrogCliBce,
		jfrogCliBp}
}

func (yg *JFrogPipelinesYamlGenerator) getUpdateCommitStatusCmd(gitResourceName string) string {
	return updateCommitStatusCmd + " " + gitResourceName
}

func (yg *JFrogPipelinesYamlGenerator) createGitResourceName() string {
	return createPipelinesSuitableName(yg.SetupData, "gitResource")
}

func (yg *JFrogPipelinesYamlGenerator) createPipelineName() string {
	return createPipelinesSuitableName(yg.SetupData, "pipeline")
}

func (yg *JFrogPipelinesYamlGenerator) createServerIdName() string {
	return createPipelinesSuitableName(yg.SetupData, "serverId")
}
