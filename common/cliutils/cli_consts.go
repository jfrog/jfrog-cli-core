package cliutils

type CommandDomain string

const (
	Rt       CommandDomain = "rt"
	Ds       CommandDomain = "ds"
	Xr       CommandDomain = "xr"
	Sbom     CommandDomain = "sbom"
	Platform CommandDomain = "platform"
)

const (
	// Common
	Threads = 3

	// Environment variables
	JfrogCliAvoidDeprecationWarnings = "JFROG_CLI_AVOID_DEPRECATION_WARNINGS"
)
