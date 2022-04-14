package coreutils

const (

	// General core constants
	OnErrorPanic OnError = "panic"

	// Common
	TokenRefreshDisabled        = 0
	TokenRefreshDefaultInterval = 60

	// Home Dir
	JfrogCertsDirName        = "certs"
	JfrogConfigFile          = "jfrog-cli.conf"
	JfrogDependenciesDirName = "dependencies"
	JfrogSecurityDirName     = "security"
	JfrogSecurityConfFile    = "security.yaml"
	JfrogBackupDirName       = "backup"
	JfrogLogsDirName         = "logs"
	JfrogLocksDirName        = "locks"
	JfrogPluginsDirName      = "plugins"
	PluginsExecDirName       = "bin"
	PluginsResourcesDirName  = "resources"
	JfrogPluginsFileName     = "plugins.yml"

	// Env
	ErrorHandling      = "JFROG_CLI_ERROR_HANDLING"
	TempDir            = "JFROG_CLI_TEMP_DIR"
	LogLevel           = "JFROG_CLI_LOG_LEVEL"
	LogTimestamp       = "JFROG_CLI_LOG_TIMESTAMP"
	ReportUsage        = "JFROG_CLI_REPORT_USAGE"
	DependenciesDir    = "JFROG_CLI_DEPENDENCIES_DIR"
	TransitiveDownload = "JFROG_CLI_TRANSITIVE_DOWNLOAD_EXPERIMENTAL"
	CI                 = "CI"
)

// Although these vars are constant, they are defined inside a vars section and not a constants section because the tests modify these values.
var (
	HomeDir     = "JFROG_CLI_HOME_DIR"
	BuildName   = "JFROG_CLI_BUILD_NAME"
	BuildNumber = "JFROG_CLI_BUILD_NUMBER"
	Project     = "JFROG_CLI_BUILD_PROJECT"
)
