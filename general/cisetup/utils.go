package cisetup

import (
	"fmt"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

const (
	m2pathCmd             = "MVN_PATH=`which mvn` && export M2_HOME=`readlink -f $MVN_PATH | xargs dirname | xargs dirname`"
	jfrogCliRtPrefix      = "jfrog rt"
	jfrogCliConfig        = "jfrog c add"
	jfrogCliOldConfig     = "jfrog rt c"
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

	urlFlag   = "url"
	rtUrlFlag = "artifactory-url"
	userFlag  = "user"

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

// Returns the JFrog CLI config command according to the given server details.
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
		"--enc-password=false",
	}, " ")
}

// Returns an array of JFrog CLI config commands according to the given CiSetupData.
func getTechConfigsCommands(serverId string, setM2ForMaven bool, data *CiSetupData) []string {
	var configs []string
	switch data.BuiltTechnology.Type {
	case coreutils.Maven:
		if setM2ForMaven {
			configs = append(configs, m2pathCmd)
		}
		configs = append(configs, getMavenConfigCmd(serverId, data.BuiltTechnology.VirtualRepo))

	case coreutils.Gradle:
		configs = append(configs, getBuildToolConfigCmd(gradleConfigCmdName, serverId, data.BuiltTechnology.VirtualRepo))

	case coreutils.Npm:
		configs = append(configs, getBuildToolConfigCmd(npmConfigCmdName, serverId, data.BuiltTechnology.VirtualRepo))

	}
	return configs
}

// Converts build tools commands to run via JFrog CLI.
func convertBuildCmd(data *CiSetupData) (buildCmd string, err error) {
	commandsArray := []string{}
	switch data.BuiltTechnology.Type {
	case coreutils.Npm:
		buildCmd, err = replaceCmdWithRegexp(data.BuiltTechnology.BuildCmd, npmInstallRegexp, npmInstallRegexpReplacement)
		if err != nil {
			return "", err
		}
		buildCmd, err = replaceCmdWithRegexp(buildCmd, npmCiRegexp, npmCiRegexpReplacement)
		if err != nil {
			return "", err
		}
	case coreutils.Maven, coreutils.Gradle:
		buildCmd, err = replaceCmdWithRegexp(data.BuiltTechnology.BuildCmd, mvnGradleRegexp, mvnGradleRegexpReplacement)
		if err != nil {
			return "", err
		}
	}
	commandsArray = append(commandsArray, buildCmd)
	return strings.Join(commandsArray, cmdAndOperator), nil
}

// Returns Maven's config command according to given server and repo information.
func getMavenConfigCmd(serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, mvnConfigCmdName,
		getFlagSyntax(serverIdResolve), serverId,
		getFlagSyntax(repoResolveReleases), repo,
		getFlagSyntax(repoResolveSnapshots), repo,
	}, " ")
}

// Returns build tool's (except Maven) config command according to given server and repo information.
func getBuildToolConfigCmd(configCmd, serverId, repo string) string {
	return strings.Join([]string{
		jfrogCliRtPrefix, configCmd,
		getFlagSyntax(serverIdResolve), serverId,
		getFlagSyntax(repoResolve), repo,
	}, " ")
}

// Returns a string of environment variable export command.
// key - The variable name.
// value - the value to be set.
func getExportCmd(key, value string) string {
	return fmt.Sprintf("export %s=%s", key, value)
}
