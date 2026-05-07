package commands

import (
	"fmt"
	"os"

	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
)

// AgentUnknown is returned when a generic AGENT env var is set but its value
// does not match any known agent. We don't propagate the raw value to keep
// metric cardinality bounded.
const AgentUnknown = "unknown"

// ExecutionContext describes how a CLI invocation was launched.
type ExecutionContext struct {
	Agent         string // e.g. "claude", "cursor", "gemini", "unknown" or "" if none
	IsAgent       bool
	IsInteractive bool   // stdout is a TTY
	TraceID       string // propagated trace ID (e.g. CURSOR_TRACE_ID), empty if none
}

// agentDetector maps an agent name to env vars whose presence proves the agent
// invoked the CLI.
type agentDetector struct {
	name string
	envs []string
}

// agentEnvDetectors is the agent detection table. First match wins.
var agentEnvDetectors = []agentDetector{
	{"claude", []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT"}},
	{"gemini", []string{"GEMINI_CLI"}},
	{"goose", []string{"GOOSE_TERMINAL"}},
	{"cursor", []string{"CURSOR_AGENT", "CURSOR_CLI", "CURSOR_TRACE_ID"}},
	{"copilot", []string{"COPILOT_CLI"}},
	{"kilocode", []string{"KILO_IPC_SOCKET_PATH", "KILO_SERVER_PASSWORD"}},
	{"roo_code", []string{"ROO_CODE_IPC_SOCKET_PATH"}},
	{"codex", []string{"CODEX_CI"}},
}

// DetectExecutionContext captures signals about who executed the CLI.
func DetectExecutionContext() ExecutionContext {
	ec := ExecutionContext{
		IsInteractive: clientlog.IsStdOutTerminal(),
	}
	ec.Agent = detectAgent()
	ec.IsAgent = ec.Agent != ""
	ec.TraceID = detectAgentTraceID(ec.Agent)
	return ec
}

func detectAgent() string {
	for _, d := range agentEnvDetectors {
		for _, e := range d.envs {
			if os.Getenv(e) != "" {
				return d.name
			}
		}
	}
	// Generic AGENT env var (goose convention, codex pending). Don't propagate the
	// raw value into metrics — collapse to "unknown" to keep cardinality bounded.
	if os.Getenv("AGENT") != "" {
		return AgentUnknown
	}
	return ""
}

// detectAgentTraceID returns a trace ID propagated by the parent agent, if any.
// Gated on agent identity to prevent stale values leaked from an outer shell
// (e.g. CURSOR_TRACE_ID present while the actual invoker is Claude Code).
// Empty result means the CLI should generate its own trace ID.
func detectAgentTraceID(agent string) string {
	if agent == "cursor" {
		return os.Getenv("CURSOR_TRACE_ID")
	}
	return ""
}

// EnrichUserAgent appends invoker context (agent and/or CI provider) to a base
// User-Agent string. Returns base unchanged when neither is detected.
// Examples: "jfrog-cli-go/2.x (claude)", "jfrog-cli-go/2.x (cursor; ci=github_actions)".
func EnrichUserAgent(base string) string {
	ec := DetectExecutionContext()
	ciSystem := detectCISystem()
	switch {
	case ec.Agent != "" && ciSystem != "":
		return fmt.Sprintf("%s (%s; ci=%s)", base, ec.Agent, ciSystem)
	case ec.Agent != "":
		return fmt.Sprintf("%s (%s)", base, ec.Agent)
	case ciSystem != "":
		return fmt.Sprintf("%s (ci=%s)", base, ciSystem)
	}
	return base
}
