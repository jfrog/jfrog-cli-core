package cisetup

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"strings"
)

const JenkinsfileName2 = "Jenkinsfile"
const resolverIdTemplate = "%s_RESOLVER"
const deployerIdTemplate = "%s_DEPLOYER"
const homeEnv = "%[1]s_HOME = '/full/path/to/%[1]s' // Set to the local %[1]s installation path."

const jenkinsfileTemplate2 = `pipeline {

	// More info about the Declarative Pipeline Syntax on https://www.jfrog.com/confluence/display/JFROG/Declarative+Pipeline+Syntax
	// Declarative syntax is available from version 3.0.0 of the Jenkins Artifactory Plugin.

	agent any

	%s

	%s
}`

const enviromentsTemplate = `
	environment {
		%s
	}`

const allStagesTemplate = `
	stages {
		%s
	}`

const stageTemplate = `
		stage (%q) {
			steps {%s
			}
		}
`

const cloneStepsTemplate = `
				git branch: %q,
				url: %q
				// credentialsId: 'git_cred_id' (If cloning the code requires credentials, set credentials to artifactory server assined in Jenkins > Configure System > credentials > "username with password" > ID: "git-cred-id" )`

const rtConfigServerStepTemplate = `
				rtServer (
					id: %[1]q,
					url: %[2]q,
					credentialsId: 'rt-cred-id', // (Credentials to artifactory server assined in Jenkins > Configure System > credentials > "username with password" > ID: "rt-cred-id" )

 					// bypassProxy: true, (If Jenkins is configured to use an http proxy, you can bypass the proxy when using this Artifactory server)
					// timeout: 300 , (Configure the connection timeout (in seconds). The default value (if not configured) is 300 seconds)
				)
				rt%[3]sDeployer (
					id: %[4]q,
					serverId: %[1]q,
					%[5]s,

					// threads: 6, (Optional - Attach custom properties to the published artifacts)
					// properties: ['key1=value1', 'key2=value2'], (Optional - Attach custom properties to the published artifacts)
				)
				rt%[3]sResolver (
					id: %[6]q,
					serverId: %[1]q,
					%[7]s
				)`

const mavenRepoTemplate = `releaseRepo: %q,
					snapshotRepo: %q`

const singleRepoTemplate = `repo: %q`

const commonBuildInfoFlags = `// buildName: 'my-build-name', (If the build name and build number are not set here, the current job name and number will be used:)
					// buildNumber: '17',
					// project: 'my-project-key' (Optional - Only if this build is associated with a project in Artifactory, set the project key as follows.)`

const mavenRunStepTemplate = `
				rtMavenRun (
					pom: 'pom.xml', // path to pom.xml file
					goals: %q,
					resolverId: %q,
					deployerId: %q,

					// tool: {build installation name}, (Maven tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > Maven installations)
					// useWrapper: true, (Set to true if you'd like the build to use the Maven Wrapper.)
					// opts: '-Xms1024m -Xmx4096m', (Optional - Maven options)
					%s
				)`

const gradleRunStepTemplate = `
				rtGradleRun (
					buildFile: 'build.gradle',
					tasks: %q,
					rootDir: "",
					resolverId: %q,
					deployerId: %q,

					// tool: {build installation name}, // Jenkins > Gradle jenkins > Global Tool Configuration > Gradle installations
					// threads: 6, (Optional - Attach custom properties to the published artifacts)
					// properties: ['key1=value1', 'key2=value2'], (Optional - Attach custom properties to the published artifacts)
					%s
				)`

const npmInstallStepTemplate = `
				rtNpmInstall (
					resolverId: %q,

					// tool: {build installation name}, (Npm tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > NodeJS installations
					// path: '',  (Optional path to the project root. If not set, the root of the workspace is assumed as the root project path.)
					// args: '--verbose',  (Optional npm flags or arguments.)
					// javaArgs: '-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005' , (Jenkins spawns a new java process during this step's execution. You have the option of passing any java args to this new process.)
					%s
				)`

const npmPublishStepTemplate = `
				rtNpmPublish (
					deployerId: %q,

					// tool: {build installation name}, (Npm tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > NodeJS installations
					// path: '',  (Optional path to the project root. If not set, the root of the workspace is assumed as the root project path.)
					// javaArgs: '-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005' , (Jenkins spawns a new java process during this step's execution. You have the option of passing any java args to this new process.)
					%s
				)`

const configBuildInfoStepsTemplate = `
				rtBuildInfo (
					captureEnv: true,
					includeEnvPatterns: ["*"],

  					// excludeEnvPatterns: ['*private*', 'internal-*'],
					%s
				)`

const publishBuildInfoStepsTemplate = `
				rtPublishBuildInfo (
					serverId: %q,

					%s
				)`

type JenkinsfileDslGenerator struct {
	SetupData *CiSetupData
}

func (jg *JenkinsfileDslGenerator) GenerateDsl() (jenkinsfileBytes []byte, jenkinsfileName string, err error) {
	serviceDetails, err := config.GetSpecificConfig(ConfigServerId, false, false)
	if err != nil {
		return nil, "", err
	}
	// Generate enviroments sections
	enviroments := generateEnviroments(string(jg.SetupData.BuiltTechnology.Type))
	// Generate Stages Section
	cloneStage := generateStage("Clone", fmt.Sprintf(cloneStepsTemplate, jg.SetupData.GitBranch, jg.SetupData.VcsCredentials.Url))
	rtConfigStage := generateStage("Artifactory configuration", generateRtConfigSteps(jg.SetupData.BuiltTechnology, serviceDetails.ArtifactoryUrl))
	execBuildStage := generateBuildStages(jg.SetupData.GetBuildCmdForNativeStep(), string(jg.SetupData.BuiltTechnology.Type))
	configBuildInfoStage := generateStage("Config build info", fmt.Sprintf(configBuildInfoStepsTemplate, commonBuildInfoFlags))
	publishBuildInfoStage := generateStage("Publish build info", fmt.Sprintf(publishBuildInfoStepsTemplate, ConfigServerId, commonBuildInfoFlags))
	// Combine all stages together
	stagesString := generateAllStages(cloneStage, rtConfigStage, execBuildStage, configBuildInfoStage, publishBuildInfoStage)

	return []byte(fmt.Sprintf(jenkinsfileTemplate2, enviroments, stagesString)), JenkinsfileName, nil
}

func generateStage(stageName, steps string) (stageString string) {
	return (fmt.Sprintf(stageTemplate, stageName, steps))
}

func generateAllStages(stages ...string) (allStagesString string) {
	allStagesString = ""
	for _, stage := range stages {
		allStagesString += stage
	}
	return (fmt.Sprintf(allStagesTemplate, allStagesString))
}

func generateEnviroments(buildType string) string {
	envs := ""
	switch buildType {
	case Maven:
		fallthrough
	case Gradle:
		envs += fmt.Sprintf(homeEnv, strings.ToUpper(buildType))
	default:
		envs += ""
	}
	if envs == "" {
		return ""
	}
	return fmt.Sprintf(enviromentsTemplate, envs)
}

func generateRtConfigSteps(techInfo *TechnologyInfo, rtUrl string) string {
	deployerRepos := ""
	resolverRepos := ""
	switch techInfo.Type {
	case Maven:
		deployerRepos = fmt.Sprintf(mavenRepoTemplate, techInfo.LocalReleasesRepo, techInfo.LocalSnapshotsRepo)
		resolverRepos = fmt.Sprintf(mavenRepoTemplate, techInfo.VirtualRepo, techInfo.VirtualRepo)
	case Gradle:
		fallthrough
	case Npm:
		deployerRepos = fmt.Sprintf(singleRepoTemplate, techInfo.LocalReleasesRepo)
		resolverRepos = fmt.Sprintf(singleRepoTemplate, techInfo.VirtualRepo)
	default:
		deployerRepos = "//Build type is not supported at the moment"
		resolverRepos = "//Build type is not supported at the moment"
	}
	buildType := string(techInfo.Type)
	resolverId := fmt.Sprintf(resolverIdTemplate, strings.ToUpper(buildType))
	deployerId := fmt.Sprintf(deployerIdTemplate, strings.ToUpper(buildType))
	return fmt.Sprintf(rtConfigServerStepTemplate, ConfigServerId, rtUrl, strings.Title(buildType), deployerId, deployerRepos, resolverId, resolverRepos)
}

func generateBuildStages(buildCmd, buildType string) (buildStages string) {
	buildStages = ""
	resolverId := fmt.Sprintf(resolverIdTemplate, strings.ToUpper(buildType))
	deployerId := fmt.Sprintf(deployerIdTemplate, strings.ToUpper(buildType))
	switch buildType {
	case Maven:
		buildStages += generateStage("Exec Maven", fmt.Sprintf(mavenRunStepTemplate, buildCmd, resolverId, deployerId, commonBuildInfoFlags))
	case Gradle:
		buildStages += generateStage("Exec Gradle", fmt.Sprintf(gradleRunStepTemplate, buildCmd, resolverId, deployerId, commonBuildInfoFlags))
	case Npm:
		buildStages += generateStage("Exec Npm install", fmt.Sprintf(npmInstallStepTemplate, resolverId, commonBuildInfoFlags))
		buildStages += generateStage("Exec Npm publish", fmt.Sprintf(npmPublishStepTemplate, deployerId, commonBuildInfoFlags))
	default:
		buildStages = "//Build type is not supported at the moment"
	}
	return buildStages
}
