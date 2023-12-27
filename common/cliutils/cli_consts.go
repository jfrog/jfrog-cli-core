package cliutils

import ()

type CommandDomain string

const (
	Rt       CommandDomain = "rt"
	Ds       CommandDomain = "ds"
	Xr       CommandDomain = "xr"
	Platform CommandDomain = "platform"
)

const (
	// Common
	Threads = 3

	// Environment variables
	JfrogCliAvoidDeprecationWarnings = "JFROG_CLI_AVOID_DEPRECATION_WARNINGS"

	// TODO: Common flags? (from CreateServerDetailsFromFlags)
)
