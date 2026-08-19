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
// AI stack (agent sessions only): Client → Agent → Model.
type ExecutionContext struct {
	// Agent: AI product that ran jf — "cursor", "claude", "copilot". Empty when not an agent.
	Agent string

	IsAgent       bool
	IsInteractive bool   // stdout is a TTY
	TraceID       string // e.g. CURSOR_TRACE_ID; empty if none

	// Client: host editor window — "cursor", "vscode", "zed", "jetbrains",
	// "windsurf", "antigravity". Empty when the session has no editor (Claude
	// Code TUI, Copilot CLI, Gemini, CI) or when the host cannot be proven.
	// Never TERM_PROGRAM / a terminal emulator.
	Client string

	// Model: model slug (JFROG_CLI_AI_MODEL) — "opus-4.7".
	Model string
}

// agentDetector maps an agent name to env signals that prove the agent
// invoked the CLI. Envs match on any non-empty value; EnvEquals requires an
// exact value (used when a var is shared with non-agent hosts). EnvContains
// matches when the named env value contains the substring (Amazon Q CLI).
type agentDetector struct {
	name        string
	envs        []string
	envEquals   map[string]string
	envContains map[string]string
}

// agentEnvDetectors is the agent detection table. First match wins.
// Signals are session markers only — not API keys, install roots, or IPC
// enablement flags that humans/IDEs also set.
//
// | Wire name   | Session signals                                              |
// |-------------|--------------------------------------------------------------|
// | grok        | GROK_AGENT=1                                                 |
// | claude      | CLAUDE_CODE_IS_COWORK, CLAUDE_CODE_CHILD_SESSION             |
// | gemini      | GEMINI_CLI                                                   |
// | goose       | GOOSE_TERMINAL                                               |
// | cursor      | CURSOR_AGENT, CURSOR_EXTENSION_HOST_ROLE=agent-exec          |
// | copilot     | COPILOT_CLI, COPILOT_AGENT, COPILOT_AGENT_JOB_ID,            |
// |             | COPILOT_AGENT_SESSION_ID                                     |
// | kilocode    | KILOCODE_FEATURE=cli                                         |
// | roo_code    | ROO_ACTIVE, ROO_CLI_RUNTIME                                  |
// | codex       | CODEX_CI, CODEX_THREAD_ID, CODEX_SANDBOX,                    |
// |             | CODEX_SANDBOX_NETWORK_DISABLED                               |
// | windsurf    | WINDSURF_CASCADE_TERMINAL                                    |
// | aider       | (AI_AGENT/AGENT only)                                        |
// | cline       | CLINE_ACTIVE                                                 |
// | opencode    | OPENCODE, OPENCODE_SESSION_ID                                |
// | amp         | AMP_CURRENT_THREAD_ID                                        |
// | augment     | AUGMENT_AGENT                                                |
// | qwen        | QWEN_CODE                                                    |
// | antigravity | ANTIGRAVITY_AGENT                                            |
// | crush       | CRUSH                                                        |
// | iflow       | IFLOW_CLI                                                    |
// | trae        | TRAE_AI_SHELL_ID                                             |
// | pi          | PI_CODING_AGENT                                              |
// | amazon_q    | AWS_EXECUTION_ENV contains AmazonQ-For-CLI                   |
// | unknown     | AI_AGENT/AGENT set to an unrecognized value                  |
//
// Client axis: host editor window, from editor-owned env — see detectClient.
// Model axis: sanitized JFROG_CLI_AI_MODEL (skill/user supplied slug).
var agentEnvDetectors = []agentDetector{
	// GROK_AGENT is also a profile *path* for humans — exact "1" only.
	{"grok", nil, map[string]string{"GROK_AGENT": "1"}, nil},
	// Cowork before generic Claude: same wire name, no extra pie slice.
	{"claude", []string{"CLAUDE_CODE_IS_COWORK"}, nil, nil},
	// CLAUDE_CODE_CHILD_SESSION is set only on tool/hook/status-line spawns
	// (Anthropic docs). CLAUDECODE / CLAUDE_CODE / CLAUDE_CODE_ENTRYPOINT are
	// omitted: IDE extensions also set them in integrated terminals (humans).
	{"claude", []string{"CLAUDE_CODE_CHILD_SESSION"}, nil, nil},
	{"gemini", []string{"GEMINI_CLI"}, nil, nil},
	{"goose", []string{"GOOSE_TERMINAL"}, nil, nil},
	// CURSOR_AGENT is Cursor's documented agent-session marker (live-verified).
	// CURSOR_EXTENSION_HOST_ROLE=agent-exec is the agent-exec extension host.
	// CURSOR_TRACE_ID / CURSOR_CLI are omitted: set for Cursor integrated
	// terminals (humans too). TRACE_ID is still read for correlation after a
	// strong cursor hit — see detectAgentTraceID.
	{"cursor", []string{"CURSOR_AGENT"}, map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "agent-exec"}, nil},
	// COPILOT_MODEL / COPILOT_ALLOW_ALL are user config flags (GitHub docs),
	// not exclusive session markers — a human shell can export them.
	{"copilot", []string{"COPILOT_CLI", "COPILOT_AGENT", "COPILOT_AGENT_JOB_ID", "COPILOT_AGENT_SESSION_ID"}, nil, nil},
	// KILOCODE_FEATURE=cli is the CLI session; KILO_PID and bare/other FEATURE
	// values fire in the VS Code extension (humans). IPC socket + password are not.
	{"kilocode", nil, map[string]string{"KILOCODE_FEATURE": "cli"}, nil},
	// ROO_ACTIVE / ROO_CLI_RUNTIME are session markers; IPC socket path is enablement.
	{"roo_code", []string{"ROO_ACTIVE", "ROO_CLI_RUNTIME"}, nil, nil},
	{"codex", []string{"CODEX_CI", "CODEX_THREAD_ID", "CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"}, nil, nil},
	// Cascade terminal marker; CODEIUM_EDITOR_APP_ROOT is IDE install (false positive).
	{"windsurf", []string{"WINDSURF_CASCADE_TERMINAL"}, nil, nil},
	// aider has no reliable session env (AIDER_API_KEY is config); AI_AGENT only.
	{"aider", []string{}, nil, nil},
	{"cline", []string{"CLINE_ACTIVE"}, nil, nil},
	// OPENCODE is the process session marker; OPENCODE_SESSION_ID is injected
	// into tool/shell child envs. OPENCODE_CLIENT is config (which client UI),
	// not a session marker — a human shell can export it.
	{"opencode", []string{"OPENCODE", "OPENCODE_SESSION_ID"}, nil, nil},
	{"amp", []string{"AMP_CURRENT_THREAD_ID"}, nil, nil},
	{"augment", []string{"AUGMENT_AGENT"}, nil, nil},
	{"qwen", []string{"QWEN_CODE"}, nil, nil},
	{"antigravity", []string{"ANTIGRAVITY_AGENT"}, nil, nil},
	{"crush", []string{"CRUSH"}, nil, nil},
	{"iflow", []string{"IFLOW_CLI"}, nil, nil},
	{"trae", []string{"TRAE_AI_SHELL_ID"}, nil, nil},
	{"pi", []string{"PI_CODING_AGENT"}, nil, nil},
	{"amazon_q", nil, nil, map[string]string{"AWS_EXECUTION_ENV": "AmazonQ-For-CLI"}},
}

// agentNameAliases maps hyphenated ecosystem spellings (e.g. @vercel/detect-agent)
// to our wire names. Identity entries for table names are not needed — see
// agentCanonical.
var agentNameAliases = map[string]string{
	"claude-code":                 "claude",
	"gemini-cli":                  "gemini",
	"cursor-cli":                  "cursor",
	"github-copilot":              "copilot",
	"copilot-cli":                 "copilot",
	"roo-code":                    "roo_code",
	"amazon-q-cli":                "amazon_q",
	"amazon-q":                    "amazon_q",
	"qwen-code":                   "qwen",
	"github_copilot_vscode_agent": "copilot",
	"grok-cli":                    "grok",
	"grok-build":                  "grok",
}

// agentCanonical is the AI_AGENT/AGENT lookup: every detector name, every alias,
// and every alias target (so amazon_q round-trips with no env detector).
var agentCanonical = buildAgentCanonical()

func buildAgentCanonical() map[string]string {
	m := make(map[string]string, len(agentEnvDetectors)+2*len(agentNameAliases))
	for _, d := range agentEnvDetectors {
		m[d.name] = d.name
	}
	for alias, canonical := range agentNameAliases {
		m[alias] = canonical
		m[canonical] = canonical
	}
	return m
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
	// Client/model only for agent sessions (human in VS Code stays unmarked).
	if ec.IsAgent {
		ec.Client = detectClient(ec.Agent)
		ec.Model = detectModel()
	}
	return ec
}

func detectModel() string {
	return sanitizeToken(os.Getenv("JFROG_CLI_AI_MODEL"))
}

// detectClient returns the editor window hosting an agent session. The host is
// read from env the *editor* owns, not inferred from the agent name: Copilot and
// Claude both run inside JetBrains, Zed and VS Code, so agent alone cannot name
// the window.
//
// | Client      | Host-owned signal                                          |
// |-------------|------------------------------------------------------------|
// | zed         | ZED_TERM                                                   |
// | jetbrains   | TERMINAL_EMULATOR=JetBrains-JediTerm                       |
// | cursor      | CURSOR_TRACE_ID, agent=cursor, or "cursor" askpass path    |
// | windsurf    | "windsurf" askpass path                                    |
// | antigravity | "antigravity" askpass path                                 |
// | vscode      | first-party Copilot plugin markers                         |
//
// Order is significant: every VS Code fork inherits TERM_PROGRAM=vscode and the
// VSCODE_* vars from upstream, so forks must resolve before anything reports
// "vscode" (product rule P13 — a Cursor session must never be labelled vscode).
// Unproven hosts omit the axis rather than guess.
//
// JetBrains publishes one terminal value for the whole family, so IntelliJ,
// GoLand and PyCharm collapse to "jetbrains"; separating them would need a
// parent-process walk on every command.
func detectClient(agent string) string {
	switch {
	case os.Getenv("ZED_TERM") != "":
		return "zed"
	case os.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm":
		return "jetbrains"
	case agent == "cursor", os.Getenv("CURSOR_TRACE_ID") != "", askpassPathContains("cursor"):
		return "cursor"
	case askpassPathContains("windsurf"):
		return "windsurf"
	case askpassPathContains("antigravity"):
		return "antigravity"
	case os.Getenv("COPILOT_AGENT") == "1", os.Getenv("AI_AGENT") == "github_copilot_vscode_agent":
		// Last resort: these prove the first-party plugin, not the window. A
		// JetBrains host is caught above, so this only fires with no host marker.
		return "vscode"
	default:
		return ""
	}
}

// askpassEnvVars hold the path of the editor's git askpass helper, which embeds
// the application name (…/Cursor.app/…, …/windsurf/…). It is the only env that
// separates a VS Code fork from upstream, since forks copy the VSCODE_* names
// verbatim. Path-shaped, so match case-insensitively on a substring.
var askpassEnvVars = []string{"VSCODE_GIT_ASKPASS_MAIN", "VSCODE_GIT_ASKPASS_NODE", "GIT_ASKPASS"}

func askpassPathContains(app string) bool {
	for _, env := range askpassEnvVars {
		if strings.Contains(strings.ToLower(os.Getenv(env)), app) {
			return true
		}
	}
	return false
}

// maxTokenLen caps sanitized identity tokens so a pathological env value cannot
// inflate User-Agent / metrics payloads. Excess is truncated after filtering.
const maxTokenLen = 64

// sanitizeToken lowercases s and keeps only [a-z0-9._-], bounding cardinality
// and guaranteeing no header-splitting sequence can reach the wire.
func sanitizeToken(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return -1
		}
	}, strings.TrimSpace(s))
	if len(s) > maxTokenLen {
		return s[:maxTokenLen]
	}
	return s
}

func detectAgent() string {
	for _, d := range agentEnvDetectors {
		for _, e := range d.envs {
			if os.Getenv(e) != "" {
				return d.name
			}
		}
		for k, v := range d.envEquals {
			if os.Getenv(k) == v {
				return d.name
			}
		}
		for k, substr := range d.envContains {
			if substr != "" && strings.Contains(os.Getenv(k), substr) {
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

// canonicalAgentName maps a generic AI_AGENT/AGENT value to a wire name.
// Returns "" for empty, a table/alias name when recognized, else AgentUnknown.
func canonicalAgentName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return ""
	}
	// Strip a version suffix, e.g. "goose@1.2.3".
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	if mapped, ok := agentCanonical[name]; ok {
		return mapped
	}
	return AgentUnknown
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
