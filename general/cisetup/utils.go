package cisetup

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
