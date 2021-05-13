package cisetup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
)

const (
	jfrogCliFullImgName   = "releases-docker.jfrog.io/jfrog/jfrog-cli-full"
	jfrogCliFullImgTag    = "latest"
	m2pathCmd             = "MVN_PATH=`which mvn` && export M2_HOME=`readlink -f $MVN_PATH | xargs dirname | xargs dirname`"
	jfrogCliRtPrefix      = "jfrog rt"
	jfrogCliConfig        = "jfrog c add"
	jfrogCliOldConfig     = "jfrog rt c"
	jfrogCliBce           = "jfrog rt bce"
	jfrogCliBag           = "jfrog rt bag"
	jfrogCliBp            = "jfrog rt bp"
	buildNameEnvVar       = "JFROG_CLI_BUILD_NAME"
	buildNumberEnvVar     = "JFROG_CLI_BUILD_NUMBER"
	buildProjectEnvVar    = "JFROG_CLI_BUILD_PROJECT"
	buildUrlEnvVar        = "JFROG_CLI_BUILD_URL"
	buildStatusEnvVar     = "JFROG_BUILD_STATUS"
	runNumberEnvVar       = "$run_number"
	projectKeyEnvVar      = "$project_key"
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

	cmdAndOperator = " &&\n"
)

func getFlagSyntax(flagName string) string {
	return fmt.Sprintf("--%s", flagName)
}

func getCdToResourceCmd(gitResourceName string) string {
	return fmt.Sprintf("cd $res_%s_resourcePath", gitResourceName)
}

func getIntDetailForCmd(intName, detail string) string {
	return fmt.Sprintf("$int_%s_%s", intName, detail)
}

func getJfrogCliConfigCmd(rtIntName, serverId string, useOld bool) string {
	usedConfigCmd := jfrogCliConfig
	usedUrlFlag := rtUrlFlag
	if useOld {
		usedConfigCmd = jfrogCliOldConfig
		usedUrlFlag = urlFlag
	}
	return strings.Join([]string{
		usedConfigCmd, serverId,
		getFlagSyntax(usedUrlFlag), getIntDetailForCmd(rtIntName, urlFlag),
		getFlagSyntax(userFlag), getIntDetailForCmd(rtIntName, userFlag),
		getFlagSyntax(apikeyFlag), getIntDetailForCmd(rtIntName, apikeyFlag),
		"--enc-password=false",
	}, " ")
}

func getTechConfigsCommands(serverId string, data *CiSetupData) []string {
	// TODO - replace DetectedTechnologies with BuiltTechnologies
	// Consider remove DetectedTechnologies for CiSetupData.
	var configs []string
	if used, ok := data.DetectedTechnologies[Maven]; ok && used {

		configs = append(configs, m2pathCmd) // TODO - discuss m2pathCmd
		configs = append(configs, getMavenConfigCmd(serverId, data.BuiltTechnologies[Maven].VirtualRepo))
	}
	if used, ok := data.DetectedTechnologies[Gradle]; ok && used {
		configs = append(configs, getBuildToolConfigCmd(gradleConfigCmdName, serverId, data.BuiltTechnologies[Gradle].VirtualRepo))
	}
	if used, ok := data.DetectedTechnologies[Npm]; ok && used {
		configs = append(configs, getBuildToolConfigCmd(npmConfigCmdName, serverId, data.BuiltTechnologies[Npm].VirtualRepo))
	}
	return configs
}

// Converts build tools commands to run via JFrog CLI.
func convertBuildCmd(data *CiSetupData) (string, error) {
	commandsArray := []string{}
	for tech, info := range data.BuiltTechnologies {
		var cmdRegexp, replacement string
		switch tech {
		case Npm:
			cmdRegexp = npmInstallRegexp
			replacement = npmInstallRegexpReplacement
		case Maven:
			fallthrough
		case Gradle:
			cmdRegexp = mvnGradleRegexp
			replacement = mvnGradleRegexpReplacement

		}
		buildCmd, err := replaceCmdWithRegexp(info.BuildCmd, cmdRegexp, replacement)
		if err != nil {
			return "", err
		}
		commandsArray = append(commandsArray, buildCmd)

	}
	return strings.Join(commandsArray, cmdAndOperator), nil
}

func getMavenConfigCmd(serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, mvnConfigCmdName,
		getFlagSyntax(serverIdResolve), serverId,
		getFlagSyntax(repoResolveReleases), repo,
		getFlagSyntax(repoResolveSnapshots), repo,
	}, " ")
}

func getBuildToolConfigCmd(configCmd, serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, configCmd,
		getFlagSyntax(serverIdResolve), serverId,
		getFlagSyntax(repoResolve), repo,
	}, " ")
}

func getExportsCommands(vcsData *CiSetupData) []string {
	return []string{
		getExportCmd(coreutils.CI, strconv.FormatBool(true)),
		getExportCmd(buildNameEnvVar, vcsData.BuildName),
		getExportCmd(buildNumberEnvVar, runNumberEnvVar),
		getExportCmd(buildUrlEnvVar, stepUrlEnvVar),
		getExportCmd(buildStatusEnvVar, passResult),
	}
}

func getExportCmd(key, value string) string {
	return fmt.Sprintf("export %s=%s", key, value)
}
