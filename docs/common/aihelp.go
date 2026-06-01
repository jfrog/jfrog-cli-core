package common

import (
	"os"
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
)

// EnvAIHelp opts a process in or out of AI-oriented help text rendering.
// Values parseable by strconv.ParseBool (1/t/true/0/f/false, case-insensitive)
// force the mode on or off. Unset or unparseable falls back to
// ExecutionContext.IsAgent auto-detection.
const EnvAIHelp = "JFROG_CLI_AI_HELP"

// AIAgentDetector reports whether the running process is an AI agent.
// The default consults the memoized ExecutionContext in
// common/commands. Exposed as a variable so tests can inject a deterministic
// answer — DetectExecutionContext caches via sync.Once and cannot be reset.
var AIAgentDetector = func() bool {
	return commands.DetectExecutionContext().IsAgent
}

// AIHelpEnabled reports whether help rendering should prefer AIDescription
// over Description. The env var, when parseable as a bool, wins over
// auto-detection so users can opt out of agent-flavored help.
func AIHelpEnabled() bool {
	if v, ok := os.LookupEnv(EnvAIHelp); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return AIAgentDetector()
}

// ResolveDescription returns the AI variant when it is non-empty and AI help
// is enabled; otherwise it returns the human variant. An empty ai always
// falls back to human, so partial backfill across commands is safe.
func ResolveDescription(human, ai string) string {
	if ai != "" && AIHelpEnabled() {
		return ai
	}
	return human
}
