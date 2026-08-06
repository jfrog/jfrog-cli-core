package commands

import (
	"os"
	"strings"
	"sync"

	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
)

// AgentUnknown is returned when a generic AGENT env var is set but its value
// does not match any known agent. We don't propagate the raw value to keep
// metric cardinality bounded.
const AgentUnknown = "unknown"

// ExecutionContext describes how a CLI invocation was launched.
type ExecutionContext struct {
	// Agent: which AI product ran jf (e.g. "cursor", "claude", "copilot").
	// Empty for humans. Same Cursor Agent in VS Code or iTerm → still "cursor".
	Agent string

	IsAgent       bool
	IsInteractive bool   // stdout is a TTY
	TraceID       string // e.g. CURSOR_TRACE_ID; empty if none

	// AIClient: which editor/terminal hosts that agent (TERM_PROGRAM), e.g.
	// "vscode", "zed", "iterm.app". Agent-only — same Cursor Agent reports
	// "vscode" in VS Code and "iterm.app" in iTerm; humans stay empty.
	AIClient string

	// AIModel: model slug from JFROG_CLI_AI_MODEL (e.g. "opus-4.7"). Agent-only.
	AIModel string
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
	{"copilot", []string{"COPILOT_CLI", "COPILOT_AGENT_SESSION_ID"}},
	{"kilocode", []string{"KILO_IPC_SOCKET_PATH", "KILO_SERVER_PASSWORD"}},
	{"roo_code", []string{"ROO_CODE_IPC_SOCKET_PATH", "ROO_ACTIVE"}},
	{"codex", []string{"CODEX_CI", "CODEX_THREAD_ID", "CODEX_SANDBOX"}},
	{"windsurf", []string{"WINDSURF_AGENT", "CODEIUM_EDITOR_APP_ROOT"}},
	{"aider", []string{"AIDER_API_KEY"}},
	{"cline", []string{"CLINE_ACTIVE"}},
	{"opencode", []string{"OPENCODE", "OPENCODE_CLIENT"}},
	{"amp", []string{"AMP_CURRENT_THREAD_ID"}},
	{"augment", []string{"AUGMENT_AGENT"}},
	{"qwen", []string{"QWEN_CODE"}},
	{"antigravity", []string{"ANTIGRAVITY_AGENT"}},
	{"crush", []string{"CRUSH"}},
	{"iflow", []string{"IFLOW_CLI"}},
	{"trae", []string{"TRAE_AI_SHELL_ID"}},
}

// agentNameAliases maps generic AI_AGENT/AGENT values whose spelling differs
// from our canonical table names (the ecosystem tends to use hyphenated ids,
// e.g. @vercel/detect-agent). Values already matching a table name resolve via
// knownAgentName and need no entry here.
var agentNameAliases = map[string]string{
	"claude-code":    "claude",
	"gemini-cli":     "gemini",
	"cursor-cli":     "cursor",
	"github-copilot": "copilot",
	"copilot-cli":    "copilot",
	"roo-code":       "roo_code",
	"amazon-q-cli":   "amazon_q",
	"amazon-q":       "amazon_q",
	"qwen-code":      "qwen",
}

// DetectExecutionContext captures signals about who executed the CLI.
// Memoized for the process lifetime so independent call sites (metrics
// collector, trace-ID setup, User-Agent enrichment) cannot diverge if a
// later caller mutates the environment.
//
// Best-effort and side-effect free: only reads env / TTY state, never returns
// an error, never logs, and must never fail or alter the success of a CLI
// command. Missing or malformed signals yield empty fields.
func DetectExecutionContext() ExecutionContext {
	executionContextOnce.Do(func() {
		cachedExecutionContext = computeExecutionContext()
	})
	return cachedExecutionContext
}

var (
	executionContextOnce   sync.Once
	cachedExecutionContext ExecutionContext
)

// ResetExecutionContextForTest clears the memoized ExecutionContext so the
// next DetectExecutionContext call re-evaluates env vars. Exported for tests
// in downstream modules (e.g. jfrog-cli) that need to assert agent-detection
// behaviour against in-process command invocations after other tests in the
// same binary have already triggered memoization.
//
// Production code MUST NOT call this. Calling it concurrently with
// DetectExecutionContext is unsafe.
func ResetExecutionContextForTest() {
	executionContextOnce = sync.Once{}
	cachedExecutionContext = ExecutionContext{}
}

func computeExecutionContext() ExecutionContext {
	ec := ExecutionContext{
		IsInteractive: clientlog.IsStdOutTerminal(),
	}
	ec.Agent = detectAgent()
	ec.IsAgent = ec.Agent != ""
	ec.TraceID = detectAgentTraceID(ec.Agent)
	// Client app and model are recorded only for agent sessions: a human running
	// the CLI from a VS Code/Zed terminal must stay indistinguishable from today.
	if ec.IsAgent {
		ec.AIClient = detectAIClient()
		ec.AIModel = detectAIModel()
	}
	return ec
}

// detectAIModel returns the AI model slug the harness advertised via
// JFROG_CLI_AI_MODEL. Env detection cannot infer the model, so the harness (or
// the jfrog setup skill) supplies it. Sanitized; empty when unset.
func detectAIModel() string {
	return sanitizeToken(os.Getenv("JFROG_CLI_AI_MODEL"))
}

// detectAIClient returns the client app (editor or terminal) hosting the AI
// agent from the TERM_PROGRAM convention (set by VS Code, Zed, Warp, iTerm,
// ...). The value is sanitized so no raw env content reaches the wire.
func detectAIClient() string {
	return sanitizeToken(os.Getenv("TERM_PROGRAM"))
}

// maxTokenLen caps sanitized identity tokens so a pathological env value cannot
// inflate User-Agent / metrics payloads. Excess is truncated after filtering.
const maxTokenLen = 64

// sanitizeToken lowercases s and keeps only [a-z0-9._-], bounding cardinality
// and guaranteeing no header-splitting sequence can reach the wire.
func sanitizeToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			if b.Len() >= maxTokenLen {
				break
			}
		}
	}
	return b.String()
}

func detectAgent() string {
	for _, d := range agentEnvDetectors {
		for _, e := range d.envs {
			if os.Getenv(e) != "" {
				return d.name
			}
		}
	}
	// Generic AI_AGENT / AGENT convention (agents.md proposal, @vercel/detect-agent):
	// the value names the agent. Honor it when recognized; any other non-empty value
	// collapses to "unknown" so a raw value never reaches metrics and cardinality
	// stays bounded.
	if name := canonicalAgentName(os.Getenv("AI_AGENT")); name != "" {
		return name
	}
	return canonicalAgentName(os.Getenv("AGENT"))
}

// canonicalAgentName maps a generic AI_AGENT/AGENT value to a known agent name.
// Returns "" for an empty value, the canonical name when recognized, or
// AgentUnknown for any other non-empty value.
func canonicalAgentName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return ""
	}
	// Strip a version suffix, e.g. "goose@1.2.3".
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	if mapped, ok := agentNameAliases[name]; ok {
		return mapped
	}
	if knownAgentName(name) {
		return name
	}
	return AgentUnknown
}

// knownAgentName reports whether name is a canonical agent id we emit on the
// wire — either a table detector name, or an alias-only id (e.g. amazon_q)
// that has no reliable env var but must still round-trip when AI_AGENT/AGENT
// already carries the canonical spelling.
func knownAgentName(name string) bool {
	for _, d := range agentEnvDetectors {
		if d.name == name {
			return true
		}
	}
	for _, mapped := range agentNameAliases {
		if mapped == name {
			return true
		}
	}
	return false
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
