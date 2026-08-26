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

// Canonical wire names reused as detector names, alias targets, and host-client
// results. Unexported: same-package tests do not need a public API.
// One-off table keys stay string literals.
const (
	nameAlacritty       = "alacritty"
	nameAmazonQ         = "amazon_q"
	nameAntigravity     = "antigravity"
	nameClaude          = "claude"
	nameCodium          = "codium"
	nameCopilot         = "copilot"
	nameCursor          = "cursor"
	nameGemini          = "gemini"
	nameGhostty         = "ghostty"
	nameGrok            = "grok"
	nameHyper           = "hyper"
	nameIterm           = "iterm"
	nameJetBrains       = "jetbrains"
	nameKitty           = "kitty"
	nameQwen            = "qwen"
	nameRooCode         = "roo_code"
	nameTerminal        = "terminal"
	nameTmux            = "tmux"
	nameTrae            = "trae"
	nameVisualStudio    = "visualstudio"
	nameVSCode          = "vscode"
	nameWarp            = "warp"
	nameWezterm         = "wezterm"
	nameWindsurf        = "windsurf"
	nameWindowsTerminal = "windows-terminal"
	nameZed             = "zed"

	envAIAgent       = "AI_AGENT"
	envAgent         = "AGENT"
	envAlacrittyLog  = "ALACRITTY_LOG"
	envCopilotAgent  = "COPILOT_AGENT"
	envCursorTraceID = "CURSOR_TRACE_ID"
	//#nosec G101 jfrog-ignore // False positive: env var name, not a credential.
	envGitAskpass       = "GIT_ASKPASS"
	envJFrogCLIAIModel  = "JFROG_CLI_AI_MODEL"
	envKittyWindowID    = "KITTY_WINDOW_ID"
	envTerm             = "TERM"
	envTerminalEmulator = "TERMINAL_EMULATOR"
	envTermProgram      = "TERM_PROGRAM"
	envTmux             = "TMUX"
	//#nosec G101 jfrog-ignore // False positive: env var name, not a credential.
	envVSCodeGitAskpassMain = "VSCODE_GIT_ASKPASS_MAIN"
	//#nosec G101 jfrog-ignore // False positive: env var name, not a credential.
	envVSCodeGitAskpassNode = "VSCODE_GIT_ASKPASS_NODE"
	envVisualStudioVersion  = "VisualStudioVersion"
	envWTSession            = "WT_SESSION"
	envZedTerm              = "ZED_TERM"

	// OS spellings and exact env values (not wire names).
	askpassVSCodium     = "vscodium"
	jediTerm            = "JetBrains-jediTerm"
	termProgramApple    = "Apple_Terminal"
	termProgramItermApp = "iTerm.app"
	termProgramWarp     = "WarpTerminal"
	termXtermGhostty    = "xterm-ghostty"

	aliasGitHubCopilotVSCodeAgent = "github_copilot_vscode_agent"
)

// ExecutionContext describes how a CLI invocation was launched.
// AI stack (agent sessions only): Client → Agent → Model.
type ExecutionContext struct {
	// Agent: AI product that ran jf — "cursor", "claude", "copilot". Empty when not an agent.
	Agent string

	IsAgent       bool
	IsInteractive bool   // stdout is a TTY
	TraceID       string // e.g. CURSOR_TRACE_ID; empty if none

	// Client: app hosting the agent session. Prefer a known IDE ("cursor",
	// "vscode", "zed", "jetbrains", "windsurf", "antigravity", "codium",
	// "trae", "visualstudio"), then a known agent app ("claude"), then a
	// short terminal name ("iterm", "warp", "terminal", "tmux",
	// "windows-terminal"). Empty when unknown.
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
// Client axis: host app, preferring editor-owned env — see detectClient.
// Model axis: sanitized JFROG_CLI_AI_MODEL (skill/user supplied slug).
var agentEnvDetectors = []agentDetector{
	// GROK_AGENT is also a profile *path* for humans — exact "1" only.
	{nameGrok, nil, map[string]string{"GROK_AGENT": "1"}, nil},
	// Cowork before generic Claude: same wire name, no extra pie slice.
	{nameClaude, []string{"CLAUDE_CODE_IS_COWORK"}, nil, nil},
	// CLAUDE_CODE_CHILD_SESSION is set only on tool/hook/status-line spawns
	// (Anthropic docs). CLAUDECODE / CLAUDE_CODE / CLAUDE_CODE_ENTRYPOINT are
	// omitted: IDE extensions also set them in integrated terminals (humans).
	{nameClaude, []string{"CLAUDE_CODE_CHILD_SESSION"}, nil, nil},
	{nameGemini, []string{"GEMINI_CLI"}, nil, nil},
	{"goose", []string{"GOOSE_TERMINAL"}, nil, nil},
	// CURSOR_AGENT is Cursor's documented agent-session marker (live-verified).
	// CURSOR_EXTENSION_HOST_ROLE=agent-exec is the agent-exec extension host.
	// CURSOR_TRACE_ID / CURSOR_CLI are omitted: set for Cursor integrated
	// terminals (humans too). TRACE_ID is still read for correlation after a
	// strong cursor hit — see detectAgentTraceID.
	{nameCursor, []string{"CURSOR_AGENT"}, map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "agent-exec"}, nil},
	// COPILOT_MODEL / COPILOT_ALLOW_ALL are user config flags (GitHub docs),
	// not exclusive session markers — a human shell can export them.
	{nameCopilot, []string{"COPILOT_CLI", envCopilotAgent, "COPILOT_AGENT_JOB_ID", "COPILOT_AGENT_SESSION_ID"}, nil, nil},
	// KILOCODE_FEATURE=cli is the CLI session; KILO_PID and bare/other FEATURE
	// values fire in the VS Code extension (humans). IPC socket + password are not.
	{"kilocode", nil, map[string]string{"KILOCODE_FEATURE": "cli"}, nil},
	// ROO_ACTIVE / ROO_CLI_RUNTIME are session markers; IPC socket path is enablement.
	{nameRooCode, []string{"ROO_ACTIVE", "ROO_CLI_RUNTIME"}, nil, nil},
	{"codex", []string{"CODEX_CI", "CODEX_THREAD_ID", "CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"}, nil, nil},
	// Cascade terminal marker; CODEIUM_EDITOR_APP_ROOT is IDE install (false positive).
	{nameWindsurf, []string{"WINDSURF_CASCADE_TERMINAL"}, nil, nil},
	// aider has no reliable session env (AIDER_API_KEY is config); AI_AGENT only.
	{"aider", []string{}, nil, nil},
	{"cline", []string{"CLINE_ACTIVE"}, nil, nil},
	// OPENCODE is the process session marker; OPENCODE_SESSION_ID is injected
	// into tool/shell child envs. OPENCODE_CLIENT is config (which client UI),
	// not a session marker — a human shell can export it.
	{"opencode", []string{"OPENCODE", "OPENCODE_SESSION_ID"}, nil, nil},
	{"amp", []string{"AMP_CURRENT_THREAD_ID"}, nil, nil},
	{"augment", []string{"AUGMENT_AGENT"}, nil, nil},
	{nameQwen, []string{"QWEN_CODE"}, nil, nil},
	{nameAntigravity, []string{"ANTIGRAVITY_AGENT"}, nil, nil},
	{"crush", []string{"CRUSH"}, nil, nil},
	{"iflow", []string{"IFLOW_CLI"}, nil, nil},
	{nameTrae, []string{"TRAE_AI_SHELL_ID"}, nil, nil},
	{"pi", []string{"PI_CODING_AGENT"}, nil, nil},
	// Amazon Q Developer CLI sets AWS_EXECUTION_ENV to a value containing
	// AmazonQ-For-CLI (AWS CLI execution-environment convention).
	{nameAmazonQ, nil, nil, map[string]string{"AWS_EXECUTION_ENV": "AmazonQ-For-CLI"}},
}

// agentNameAliases maps hyphenated ecosystem spellings (e.g. @vercel/detect-agent)
// to our wire names. Identity entries for table names are not needed — see
// agentCanonical.
var agentNameAliases = map[string]string{
	"claude-code":                 nameClaude,
	"gemini-cli":                  nameGemini,
	"cursor-cli":                  nameCursor,
	"github-copilot":              nameCopilot,
	"copilot-cli":                 nameCopilot,
	"roo-code":                    nameRooCode,
	"amazon-q-cli":                nameAmazonQ,
	"amazon-q":                    nameAmazonQ,
	"qwen-code":                   nameQwen,
	aliasGitHubCopilotVSCodeAgent: nameCopilot,
	"grok-cli":                    nameGrok,
	"grok-build":                  nameGrok,
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
	return sanitizeWireName(os.Getenv(envJFrogCLIAIModel))
}

// detectClient returns the app hosting an agent session. It starts with
// editor-owned env because Copilot and Claude can run inside JetBrains, Zed,
// Cursor and VS Code. If no IDE is proven, a known standalone app wins, then
// TERM_PROGRAM is mapped to a short product name the same way IDEs are.
//
// | Client          | Host-owned signal                                              |
// |-----------------|----------------------------------------------------------------|
// | zed             | ZED_TERM                                                       |
// | jetbrains       | TERMINAL_EMULATOR=JetBrains-jediTerm                           |
// | cursor          | CURSOR_TRACE_ID, or Cursor.app /cursor/resources / .cursor-server |
// | windsurf        | agent=windsurf or Windsurf.app /windsurf/resources / .windsurf-server           |
// | antigravity     | agent=antigravity or Antigravity.app /antigravity/resources / .antigravity-server |
// | trae            | agent=trae or Trae.app /trae/resources / .trae-server                           |
// | codium          | VSCodium.app /vscodium/resources /codium/resources / .vscodium-server            |
// | visualstudio    | VisualStudioVersion (desktop VS, not VS Code)                  |
// | vscode          | stock VS Code askpass, or Copilot plugin with no terminal      |
// | claude          | agent=claude after IDE checks                                  |
// | iterm/warp/…    | TERM_PROGRAM, else TMUX / WT_SESSION / TERM=xterm-ghostty      |
//
// Order is significant: every VS Code fork inherits TERM_PROGRAM=vscode and the
// VSCODE_* vars from upstream, so forks must resolve before anything reports
// "vscode" (product rule P13 — a Cursor session must never be labelled vscode).
//
// JetBrains publishes one terminal value for the whole family, so IntelliJ,
// GoLand and PyCharm collapse to "jetbrains"; separating them would need a
// parent-process walk on every command. Do not split the family via macOS
// bundle IDs (peers that do that also mis-label jediTerm as PyCharm).
func detectClient(agent string) string {
	switch {
	case os.Getenv(envZedTerm) != "":
		return nameZed
	case os.Getenv(envTerminalEmulator) == jediTerm:
		return nameJetBrains
	case os.Getenv(envCursorTraceID) != "", askpassPathContains(nameCursor):
		// CURSOR_AGENT does not imply the window: the standalone CLI sets it
		// in iTerm, stock VS Code, and CI. Trace ID / askpass prove Cursor.
		return nameCursor
	case agent == nameWindsurf, askpassPathContains(nameWindsurf):
		return nameWindsurf
	case agent == nameAntigravity, askpassPathContains(nameAntigravity):
		return nameAntigravity
	case agent == nameTrae, askpassPathContains(nameTrae):
		return nameTrae
	case askpassPathContains(askpassVSCodium), askpassPathContains(nameCodium):
		return nameCodium
	case os.Getenv(envVisualStudioVersion) != "":
		return nameVisualStudio
	case askpassLooksLikeStockVSCode():
		// Positive VS Code install only. An unrecognized fork (Cursor Nightly.app)
		// must not fall through as vscode.
		return nameVSCode
	case os.Getenv(envCopilotAgent) == "1", isCopilotVSCodePluginAlias():
		// Plugin markers are not a window. A known terminal wins; vscode is
		// only the no-terminal fallback (extension-host child with no TERM_PROGRAM).
		if name := hostTerminalName(); name != "" {
			return name
		}
		return nameVSCode
	case agent == nameClaude:
		return nameClaude
	default:
		return hostTerminalName()
	}
}

func hostTerminalName() string {
	if name := canonicalTerminalName(os.Getenv(envTermProgram)); name != "" {
		return name
	}
	return fallbackTerminalName()
}

// terminalNameAliases maps sanitized TERM_PROGRAM values to short product
// names, matching the IDE style (cursor, vscode) rather than raw env spellings.
var terminalNameAliases = map[string]string{
	sanitizeWireName(termProgramItermApp): nameIterm,
	nameIterm:                             nameIterm,
	sanitizeWireName(termProgramApple):    nameTerminal,
	sanitizeWireName(termProgramWarp):     nameWarp,
	nameWarp:                              nameWarp,
	nameTmux:                              nameTmux,
	nameWezterm:                           nameWezterm,
	nameAlacritty:                         nameAlacritty,
	nameKitty:                             nameKitty,
	nameGhostty:                           nameGhostty,
	nameHyper:                             nameHyper,
}

func canonicalTerminalName(raw string) string {
	name := sanitizeWireName(raw)
	if name == "" {
		return ""
	}
	if mapped, ok := terminalNameAliases[name]; ok {
		return mapped
	}
	name = strings.TrimSuffix(name, ".app")
	if mapped, ok := terminalNameAliases[name]; ok {
		return mapped
	}
	// Unmapped values (including inherited TERM_PROGRAM=vscode) stay empty so
	// the client label stays on the known-app allowlist.
	return ""
}

// fallbackTerminalName is used when TERM_PROGRAM is missing or is the inherited
// vscode value that canonicalTerminalName refuses. These vars name a real app
// (tmux, Windows Terminal, Ghostty, Kitty, Alacritty) without proving vscode.
func fallbackTerminalName() string {
	switch {
	case os.Getenv(envTmux) != "":
		return nameTmux
	case os.Getenv(envWTSession) != "":
		return nameWindowsTerminal
	case os.Getenv(envTerm) == termXtermGhostty:
		return nameGhostty
	case os.Getenv(envKittyWindowID) != "":
		return nameKitty
	case os.Getenv(envAlacrittyLog) != "":
		return nameAlacritty
	default:
		return ""
	}
}

// askpassEnvVars hold the path of the editor's git askpass helper, which embeds
// the application name (…/Cursor.app/…, …/windsurf/resources/…). It is the only
// env that separates a VS Code fork from upstream, since forks copy the VSCODE_*
// names verbatim. Generic GIT_ASKPASS is excluded because arbitrary paths can
// contain an app name without proving the host.
var askpassEnvVars = []string{envVSCodeGitAskpassMain, envVSCodeGitAskpassNode}

// askpassPathContains reports whether a VS Code-fork askpass path is that
// editor's install, not an unrelated substring. A Windows VS Code path lives
// under the user profile, so a login named "cursor" would otherwise match.
// Remote-SSH installs use ~/.cursor-server (and the same .*-server layout
// on other forks); those must not fall through to client=vscode.
func askpassPathContains(app string) bool {
	needle := strings.ToLower(app)
	for _, env := range askpassEnvVars {
		p := strings.ToLower(strings.ReplaceAll(os.Getenv(env), "\\", "/"))
		if p == "" {
			continue
		}
		if strings.Contains(p, "/"+needle+".app") ||
			strings.Contains(p, "/"+needle+"/resources") ||
			// Remote-SSH installs live under ~/.cursor-server (and the same
			// .*-server layout on other VS Code forks).
			strings.Contains(p, "/."+needle+"-server") {
			return true
		}
	}
	return false
}

// askpassLooksLikeStockVSCode reports a proven upstream VS Code install, not
// merely a non-empty VSCODE_GIT_ASKPASS_* var (every fork sets those).
func askpassLooksLikeStockVSCode() bool {
	for _, env := range askpassEnvVars {
		p := strings.ToLower(strings.ReplaceAll(os.Getenv(env), "\\", "/"))
		if p == "" {
			continue
		}
		if strings.Contains(p, "/visual studio code") ||
			strings.Contains(p, "/microsoft vs code") ||
			strings.Contains(p, "/.vscode-server") ||
			strings.Contains(p, "/code/resources") {
			return true
		}
	}
	return false
}

// maxWireNameLen caps client/model/terminal names so a pathological env value
// cannot inflate User-Agent / metrics payloads. Excess is truncated after filtering.
const maxWireNameLen = 64

// sanitizeWireName lowercases s and keeps only [a-z0-9._-], bounding cardinality
// and guaranteeing no header-splitting sequence can reach the wire.
func sanitizeWireName(s string) string {
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
	if len(s) > maxWireNameLen {
		return s[:maxWireNameLen]
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
	if name := canonicalAgentName(os.Getenv(envAIAgent)); name != "" {
		return name
	}
	return canonicalAgentName(os.Getenv(envAgent))
}

// foldAgentName lowercases and trims a generic AI_AGENT/AGENT value and
// strips a version suffix (e.g. "goose@1.2.3"). Empty input stays empty.
func foldAgentName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return ""
	}
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	return name
}

// isCopilotVSCodePluginAlias reports the github_copilot_vscode_agent spelling
// on AI_AGENT or AGENT after the same fold detectAgent uses. Do not reuse
// canonicalAgentName here: that also maps copilot / copilot-cli, which are
// the CLI and must not force client=vscode.
func isCopilotVSCodePluginAlias() bool {
	return foldAgentName(os.Getenv(envAIAgent)) == aliasGitHubCopilotVSCodeAgent ||
		foldAgentName(os.Getenv(envAgent)) == aliasGitHubCopilotVSCodeAgent
}

// canonicalAgentName maps a generic AI_AGENT/AGENT value to a wire name.
// Returns "" for empty, a table/alias name when recognized, else AgentUnknown.
func canonicalAgentName(raw string) string {
	name := foldAgentName(raw)
	if name == "" {
		return ""
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
	if agent == nameCursor {
		return os.Getenv(envCursorTraceID)
	}
	return ""
}
