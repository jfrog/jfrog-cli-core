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
	ErrorHandling      = "JFROG_CLI_ERROR_HANDLING"
	TempDir            = "JFROG_CLI_TEMP_DIR"
	LogLevel           = "JFROG_CLI_LOG_LEVEL"
	LogTimestamp       = "JFROG_CLI_LOG_TIMESTAMP"
	ReportUsage        = "JFROG_CLI_REPORT_USAGE"
	DependenciesDir    = "JFROG_CLI_DEPENDENCIES_DIR"
	TransitiveDownload = "JFROG_CLI_TRANSITIVE_DOWNLOAD_EXPERIMENTAL"
	FailNoOp           = "JFROG_CLI_FAIL_NO_OP"
	CI                 = "CI"
	ServerID           = "JFROG_CLI_SERVER_ID"
)

// Although these vars are constant, they are defined inside a vars section and not a constants section because the tests modify these values.
var (
	HomeDir     = "JFROG_CLI_HOME_DIR"
	BuildName   = "JFROG_CLI_BUILD_NAME"
	BuildNumber = "JFROG_CLI_BUILD_NUMBER"
	Project     = "JFROG_CLI_BUILD_PROJECT"
	//#nosec G101
	EncryptionKey = "JFROG_CLI_ENCRYPTION_KEY"
)
