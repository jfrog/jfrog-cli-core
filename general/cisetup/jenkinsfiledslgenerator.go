package cisetup

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/general/techindicators"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"strings"
)

const (
	JenkinsDslFileName = "Jenkinsfile"
	resolverIdTemplate = "%s_RESOLVER"
	deployerIdTemplate = "%s_DEPLOYER"
	homeEnv            = "%[1]s_HOME = '/full/path/to/%[1]s' // Set to the local %[1]s installation path."

	jenkinsfileTemplate2 = `pipeline {

	// More info about the Declarative Pipeline Syntax on https://www.jfrog.com/confluence/display/JFROG/Declarative+Pipeline+Syntax
	// Declarative syntax is available from version 3.0.0 of the Jenkins Artifactory Plugin.

	agent any

	%s

	%s
}`

	environmentsTemplate = `
	environment {
		%s
	}`

	allStagesTemplate = `
	stages {
		%s
	}`

	stageTemplate = `
		stage (%q) {
			steps {%s
			}
		}
`

	cloneStepsTemplate = `
				git branch: %q,
				url: %q
				// credentialsId: 'git_cred_id'. If cloning the code requires credentials, set the credentials to your git in Jenkins > Configure System > credentials > "username with password" > ID: "git-cred-id"`

	rtConfigServerStepTemplate = `
				rtServer (
					id: %[1]q,
					url: %[2]q,
					credentialsId: 'rt-cred-id', // Set the credentials to your JFrog instance in Jenkins > Configure System > credentials > "username with password" > ID: "rt-cred-id"

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

	mavenRepoTemplate = `releaseRepo: %q,
					snapshotRepo: %q`

	singleRepoTemplate = `repo: %q`

	mavenRunStepTemplate = `
				rtMavenRun (
					pom: 'pom.xml', // path to pom.xml file
					goals: %q,
					resolverId: %q,
					deployerId: %q,

					// tool: {build installation name}, (Maven tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > Maven installations)
					// useWrapper: true, (Set to true if you'd like the build to use the Maven Wrapper.)
					// opts: '-Xms1024m -Xmx4096m', (Optional - Maven options)
				)`

	gradleRunStepTemplate = `
				rtGradleRun (
					buildFile: 'build.gradle',
					tasks: %q,
					rootDir: "",
					resolverId: %q,
					deployerId: %q,

					// tool: {build installation name}, // Jenkins > Gradle jenkins > Global Tool Configuration > Gradle installations
				)`

	npmInstallStepTemplate = `
				rtNpmInstall (
					resolverId: %q,

					// tool: {build installation name}, (Npm tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > NodeJS installations
				)`

	npmPublishStepTemplate = `
				rtNpmPublish (
					deployerId: %q,

					// tool: {build installation name}, (Npm tool installation from jenkins from : Jenkins > Manage jenkins > Global Tool Configuration > NodeJS installations
					// path: '',  (Optional path to the project root. If not set, the root of the workspace is assumed as the root project path.)
					// javaArgs: '-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005' , (Jenkins spawns a new java process during this step's execution. You have the option of passing any java args to this new process.)
				)`

	publishBuildInfoStepsTemplate = `
				rtPublishBuildInfo (
					serverId: %q,
				)`
)

type JenkinsfileDslGenerator struct {
	SetupData *CiSetupData
}

func (jg *JenkinsfileDslGenerator) GenerateDsl() (jenkinsfileBytes []byte, jenkinsfileName string, err error) {
	serviceDetails, err := config.GetSpecificConfig(ConfigServerId, false, false)
	if err != nil {
		return nil, "", err
	}
	// Generate environments sections
	environments := generateEnvironments(string(jg.SetupData.BuiltTechnology.Type))
	// Generate Stages Section
	cloneStage := generateStage("Clone", fmt.Sprintf(cloneStepsTemplate, jg.SetupData.GitBranch, jg.SetupData.VcsCredentials.Url))
	rtConfigStage := generateStage("Artifactory configuration", generateRtConfigSteps(jg.SetupData.BuiltTechnology, serviceDetails.ArtifactoryUrl))
	execBuildStage := generateBuildStages(jg.SetupData.GetBuildCmdForNativeStep(), string(jg.SetupData.BuiltTechnology.Type))
	publishBuildInfoStage := generateStage("Publish build info", fmt.Sprintf(publishBuildInfoStepsTemplate, ConfigServerId))
	// Combine all stages together
	stagesString := generateAllStages(cloneStage, rtConfigStage, execBuildStage, publishBuildInfoStage)

	return []byte(fmt.Sprintf(jenkinsfileTemplate2, environments, stagesString)), JenkinsDslFileName, nil
}

func generateStage(stageName, steps string) (stageString string) {
	return fmt.Sprintf(stageTemplate, stageName, steps)
}

func generateAllStages(stages ...string) (allStagesString string) {
	allStagesString = ""
	for _, stage := range stages {
		allStagesString += stage
	}
	return fmt.Sprintf(allStagesTemplate, allStagesString)
}

func generateEnvironments(buildType string) string {
	envs := ""
	switch buildType {
	case techindicators.Maven:
		fallthrough
	case techindicators.Gradle:
		envs += fmt.Sprintf(homeEnv, strings.ToUpper(buildType))
	default:
		envs += ""
	}
	if envs == "" {
		return ""
	}
	return fmt.Sprintf(environmentsTemplate, envs)
}

func generateRtConfigSteps(techInfo *TechnologyInfo, rtUrl string) string {
	deployerRepos := ""
	resolverRepos := ""
	switch techInfo.Type {
	case techindicators.Maven:
		deployerRepos = fmt.Sprintf(mavenRepoTemplate, techInfo.LocalReleasesRepo, techInfo.LocalSnapshotsRepo)
		resolverRepos = fmt.Sprintf(mavenRepoTemplate, techInfo.VirtualRepo, techInfo.VirtualRepo)
	case techindicators.Gradle:
		fallthrough
	case techindicators.Npm:
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
	case techindicators.Maven:
		buildStages += generateStage("Exec Maven", fmt.Sprintf(mavenRunStepTemplate, buildCmd, resolverId, deployerId))
	case techindicators.Gradle:
		buildStages += generateStage("Exec Gradle", fmt.Sprintf(gradleRunStepTemplate, buildCmd, resolverId, deployerId))
	case techindicators.Npm:
		buildStages += generateStage("Exec Npm install", fmt.Sprintf(npmInstallStepTemplate, resolverId))
		buildStages += generateStage("Exec Npm publish", fmt.Sprintf(npmPublishStepTemplate, deployerId))
	default:
		buildStages = "//Build type is not supported at the moment"
	}
	return buildStages
}
