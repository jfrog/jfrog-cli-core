package coreutils

const (

	// General core constants
	OnErrorPanic OnError = "panic"

	// Common
	TokenRefreshDisabled        = 0
	TokenRefreshDefaultInterval = 60

	// Home Dir
	JfrogBackupDirName                  = "backup"
	JfrogCertsDirName                   = "certs"
	JfrogConfigFile                     = "jfrog-cli.conf"
	JfrogDependenciesDirName            = "dependencies"
	JfrogLocksDirName                   = "locks"
	JfrogLogsDirName                    = "logs"
	JfrogPluginsDirName                 = "plugins"
	JfrogPluginsFileName                = "plugins.yml"
	JfrogSecurityConfFile               = "security.yaml"
	JfrogSecurityDirName                = "security"
	JfrogTransferDelaysDirName          = "delays"
	JfrogTransferDirName                = "transfer"
	JfrogTransferErrorsDirName          = "errors"
	JfrogTransferRepoSnapshotFileName   = "repo-snapshot.json"
	JfrogTransferRepoStateFileName      = "repo-state.json"
	JfrogTransferRepositoriesDirName    = "repositories"
	JfrogTransferTempDirName            = "tmp"
	JfrogTransferRetryableErrorsDirName = "retryable"
	JfrogTransferRunStatusFileName      = "run-status.json"
	JfrogTransferSkippedErrorsDirName   = "skipped"
	JfrogTransferSnapshotDirName        = "snapshot"
	JfrogTransferStateFileName          = "state.json"
	PluginsExecDirName                  = "bin"
	PluginsResourcesDirName             = "resources"
	//#nosec G101
	CurationPassThroughApi = "api/curation/audit/"

	//#nosec G101
	ErrorHandling           = "JFROG_CLI_ERROR_HANDLING"
	TempDir                 = "JFROG_CLI_TEMP_DIR"
	LogLevel                = "JFROG_CLI_LOG_LEVEL"
	LogTimestamp            = "JFROG_CLI_LOG_TIMESTAMP"
	ReportUsage             = "JFROG_CLI_REPORT_USAGE"
	DependenciesDir         = "JFROG_CLI_DEPENDENCIES_DIR"
	FailNoOp                = "JFROG_CLI_FAIL_NO_OP"
	SummaryOutputDirPathEnv = "JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR"
	CI                      = "CI"
	ServerID                = "JFROG_CLI_SERVER_ID"
	TransitiveDownload      = "JFROG_CLI_TRANSITIVE_DOWNLOAD"
	// Token provided by the OIDC provider, used to exchange for an access token.
	//#nosec G101 // False positive: This is not a hardcoded credential.
	OidcExchangeTokenId = "JFROG_CLI_OIDC_EXCHANGE_TOKEN_ID"
	OidcProviderType    = "JFROG_CLI_OIDC_PROVIDER_TYPE"
	// These environment variables are used to adjust command names for more detailed tracking in the usage report.
	// Set by the setup-jfrog-cli GitHub Action to identify specific command usage scenarios.
	// True if an automatic build publication was triggered.
	UsageAutoPublishedBuild = "JFROG_CLI_USAGE_AUTO_BUILD_PUBLISHED"

	// Deprecated and replaced with TransitiveDownload
	TransitiveDownloadExperimental = "JFROG_CLI_TRANSITIVE_DOWNLOAD_EXPERIMENTAL"
)

// Although these vars are constant, they are defined inside a vars section and not a constants section because the tests modify these values.
var (
	HomeDir              = "JFROG_CLI_HOME_DIR"
	BuildName            = "JFROG_CLI_BUILD_NAME"
	BuildNumber          = "JFROG_CLI_BUILD_NUMBER"
	BuildUrl             = "JFROG_CLI_BUILD_URL"
	EnvExclude           = "JFROG_CLI_ENV_EXCLUDE"
	Project              = "JFROG_CLI_BUILD_PROJECT"
	ApplicationKey       = "JFROG_CLI_APPLICATION_KEY"
	SourceCodeRepository = "JFROG_CLI_SOURCECODE_REPOSITORY"
	SigningKey           = "JFROG_CLI_SIGNING_KEY"
	KeyAlias             = "JFROG_CLI_KEY_ALIAS"
	//#nosec G101
	EncryptionKey = "JFROG_CLI_ENCRYPTION_KEY"
	// For CI runs
	CIJobID       = "JFROG_CLI_CI_JOB_ID"
	CIRunID       = "JFROG_CLI_CI_RUN_ID"
	CIVcsUrl      = "JFROG_CLI_CI_VCS_URL"
	CIVcsRevision = "JFROG_CLI_CI_VCS_REVISION"
	CIVcsBranch   = "JFROG_CLI_CI_BRANCH"
)
