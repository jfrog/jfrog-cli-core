package commands

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectAgent_FromTable(t *testing.T) {
	for _, d := range agentEnvDetectors {
		for _, env := range d.envs {
			t.Run(env, func(t *testing.T) {
				clearAgentEnvVars(t)
				t.Setenv(env, "1")
				assert.Equal(t, d.name, detectAgent())
			})
		}
		for k, v := range d.envEquals {
			t.Run(k+"="+v, func(t *testing.T) {
				clearAgentEnvVars(t)
				t.Setenv(k, v)
				assert.Equal(t, d.name, detectAgent())
			})
		}
		for k, substr := range d.envContains {
			t.Run(k+" contains "+substr, func(t *testing.T) {
				clearAgentEnvVars(t)
				t.Setenv(k, "prefix-"+substr+"-suffix")
				assert.Equal(t, d.name, detectAgent())
			})
		}
	}
}

func TestDetectAgent_EnvEqualsRequiresExactValue(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("CURSOR_EXTENSION_HOST_ROLE", "extension-host")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_CursorCLINotASessionMarker(t *testing.T) {
	// CURSOR_CLI is set for Cursor integrated terminals (humans included).
	clearAgentEnvVars(t)
	t.Setenv("CURSOR_CLI", "1")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_CursorTraceIDNotASessionMarker(t *testing.T) {
	// CURSOR_TRACE_ID is also set for regular Cursor integrated terminals.
	clearAgentEnvVars(t)
	t.Setenv(EnvCursorTraceID, "trace-123")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_CopilotConfigNotASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("COPILOT_MODEL", "gpt-5.2")
	assert.Equal(t, "", detectAgent())
	t.Setenv("COPILOT_ALLOW_ALL", "true")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_OpenCodeClientNotASessionMarker(t *testing.T) {
	// OPENCODE_CLIENT names which OpenCode UI is configured; humans can export it.
	clearAgentEnvVars(t)
	t.Setenv("OPENCODE_CLIENT", "cli")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_ClaudeIDETerminalNotASessionMarker(t *testing.T) {
	// CLAUDECODE / CLAUDE_CODE / CLAUDE_CODE_ENTRYPOINT leak into IDE
	// integrated terminals where humans run CLIs directly.
	clearAgentEnvVars(t)
	t.Setenv("CLAUDECODE", "1")
	assert.Equal(t, "", detectAgent())
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "1")
	assert.Equal(t, "", detectAgent())
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_RemovedIPCNotASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("KILO_IPC_SOCKET_PATH", "/tmp/kilo.sock")
	t.Setenv("KILO_SERVER_PASSWORD", "x")
	assert.Equal(t, "", detectAgent())
	t.Setenv("KILO_IPC_SOCKET_PATH", "")
	t.Setenv("KILO_SERVER_PASSWORD", "")
	t.Setenv("ROO_CODE_IPC_SOCKET_PATH", "/tmp/roo.sock")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_KiloExtensionNotASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("KILO_PID", "12345")
	t.Setenv("KILOCODE_FEATURE", "vscode-extension")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_KiloCLIIsASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("KILOCODE_FEATURE", "cli")
	assert.Equal(t, "kilocode", detectAgent())
}

func TestDetectAgent_GrokAgentPathIsNotASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("GROK_AGENT", "/Users/me/.grok/profile")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_CoworkEmitsClaude(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_IS_COWORK", "1")
	assert.Equal(t, NameClaude, detectAgent())
}

func TestDetectAgent_GenericAgentEnvCollapsesToUnknown(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("AGENT", "some_random_value")
	assert.Equal(t, AgentUnknown, detectAgent())
}

func TestDetectAgent_GenericValueMapsToKnownAgent(t *testing.T) {
	cases := map[string]string{
		"claude-code":                 NameClaude,
		"gemini-cli":                  NameGemini,
		"github-copilot":              NameCopilot,
		"roo-code":                    NameRooCode,
		NameRooCode:                   NameRooCode, // canonical form round-trips
		NameQwen:                      NameQwen,
		"amazon-q-cli":                NameAmazonQ,
		NameAmazonQ:                   NameAmazonQ,
		"aider":                       "aider",
		"goose@1.2.3":                 "goose",
		"CURSOR":                      NameCursor,
		AliasGitHubCopilotVSCodeAgent: NameCopilot,
		"grok-cli":                    NameGrok,
		"grok-build":                  NameGrok,
		"totally-made-up":             AgentUnknown,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			clearAgentEnvVars(t)
			t.Setenv(EnvAIAgent, value)
			assert.Equal(t, want, detectAgent())
		})
	}
}

// Table order is first-match-wins; gemini precedes cursor so a shell that
// leaked both signals (nested agents / shared env) resolves identically to
// the jfrog-skills detect_harness table.
func TestDetectAgent_TableOrderGeminiBeforeCursor(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("GEMINI_CLI", "1")
	t.Setenv("CURSOR_AGENT", "1")
	assert.Equal(t, NameGemini, detectAgent())
}

func TestDetectAgent_TableWinsOverGenericValue(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv(EnvAIAgent, NameCursor)
	assert.Equal(t, NameClaude, detectAgent())
}

func TestDetectAgent_None(t *testing.T) {
	clearAgentEnvVars(t)
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgentTraceID(t *testing.T) {
	t.Setenv(EnvCursorTraceID, "trace-abc")
	assert.Equal(t, "trace-abc", detectAgentTraceID(NameCursor))
	// Trace ID gated on agent identity: a leaked CURSOR_TRACE_ID from an outer
	// shell must not be reused when the real invoker is a different agent.
	assert.Equal(t, "", detectAgentTraceID(NameClaude))
	assert.Equal(t, "", detectAgentTraceID(""))
}

func TestDetectExecutionContext_Agent(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")

	ec := DetectExecutionContext()
	assert.True(t, ec.IsAgent)
	assert.Equal(t, NameClaude, ec.Agent)
}

func TestDetectExecutionContext_NoEnv(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Agent)
	assert.Equal(t, "", ec.TraceID)
}

func TestDetectExecutionContext_IsMemoized(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	first := DetectExecutionContext()

	// Mutate env after first call; result must not change without reset.
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "")
	t.Setenv("CURSOR_AGENT", "1")
	second := DetectExecutionContext()

	assert.Equal(t, first, second)
	assert.Equal(t, NameClaude, second.Agent)
}

// resetExecutionContextForTest forces the next DetectExecutionContext call to
// re-evaluate env vars. Restores the memoization state after the test.
func resetExecutionContextForTest(t *testing.T) {
	t.Helper()
	prevCache := cachedExecutionContext
	executionContextOnce = sync.Once{}
	cachedExecutionContext = ExecutionContext{}
	t.Cleanup(func() {
		executionContextOnce = sync.Once{}
		cachedExecutionContext = prevCache
	})
}

func clearAgentEnvVars(t *testing.T) {
	t.Helper()
	for _, d := range agentEnvDetectors {
		for _, e := range d.envs {
			t.Setenv(e, "")
		}
		for k := range d.envEquals {
			t.Setenv(k, "")
		}
		for k := range d.envContains {
			t.Setenv(k, "")
		}
	}
	t.Setenv("AGENT", "")
	t.Setenv(EnvAIAgent, "")
	// Cleared even though no longer detectors — leftover process env must not
	// bleed into tests that assert human / strong-signal behaviour.
	t.Setenv(EnvCursorTraceID, "")
	t.Setenv("CURSOR_CLI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")
	t.Setenv("COPILOT_MODEL", "")
	t.Setenv("COPILOT_ALLOW_ALL", "")
	t.Setenv("OPENCODE_CLIENT", "")
	t.Setenv("KILO_IPC_SOCKET_PATH", "")
	t.Setenv("KILO_SERVER_PASSWORD", "")
	t.Setenv("ROO_CODE_IPC_SOCKET_PATH", "")
	t.Setenv("KILO_PID", "")
	t.Setenv("KILOCODE_FEATURE", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "")
	t.Setenv("TMUX", "")
	t.Setenv("WT_SESSION", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("ALACRITTY_LOG", "")
	t.Setenv("JFROG_CLI_AI_MODEL", "")
	t.Setenv(EnvCopilotAgent, "")
	t.Setenv("COPILOT_AGENT_JOB_ID", "")
	t.Setenv(EnvVisualStudioVersion, "")
	// Host-window signals: a developer running these tests from a Zed, JetBrains
	// or Cursor terminal must not have their editor leak into client assertions.
	t.Setenv("ZED_TERM", "")
	t.Setenv("TERMINAL_EMULATOR", "")
	t.Setenv("GIT_ASKPASS", "")
	for _, env := range askpassEnvVars {
		t.Setenv(env, "")
	}
}

func TestDetectExecutionContext_ModelAgentOnly(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("JFROG_CLI_AI_MODEL", "opus-4.7")

	assert.Equal(t, "opus-4.7", DetectExecutionContext().Model)
}

func TestDetectExecutionContext_ModelSkippedForHuman(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("JFROG_CLI_AI_MODEL", "opus-4.7")

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Model)
}

func TestSanitizeToken(t *testing.T) {
	assert.Equal(t, NameVSCode, sanitizeToken(NameVSCode))
	assert.Equal(t, "apple_terminal", sanitizeToken("Apple_Terminal"))
	assert.Equal(t, "iterm.app", sanitizeToken("  iTerm.app  "))
	assert.Equal(t, "1.2.3-beta", sanitizeToken("1.2.3-beta"))
	// Header-splitting and stray characters (CR/LF, colon, spaces) are stripped.
	assert.Equal(t, "xyz", sanitizeToken("x\r\n y: z"))
	assert.Equal(t, "", sanitizeToken(""))
	// Pathological env values are truncated so they cannot inflate the wire payload.
	assert.Equal(t, maxTokenLen, len(sanitizeToken(strings.Repeat("a", maxTokenLen+100))))
}

func TestDetectExecutionContext_ClientCursorIgnoresTermProgram(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CURSOR_AGENT", "1")
	t.Setenv("TERM_PROGRAM", NameVSCode)

	ec := DetectExecutionContext()
	assert.Equal(t, NameCursor, ec.Agent)
	assert.Equal(t, NameCursor, ec.Client)
}

func TestDetectExecutionContext_ClientClaudeAppWithoutKnownIDE(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("TERM_PROGRAM", "iTerm.app")

	ec := DetectExecutionContext()
	assert.Equal(t, NameClaude, ec.Agent)
	assert.Equal(t, NameClaude, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotVscodePlugin(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv(EnvCopilotAgent, "1")
	t.Setenv("TERM_PROGRAM", "iTerm.app")

	ec := DetectExecutionContext()
	assert.Equal(t, NameCopilot, ec.Agent)
	assert.Equal(t, NameVSCode, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotCLIFallsBackToTerminalApp(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("COPILOT_CLI", "1")
	t.Setenv("TERM_PROGRAM", "iTerm.app")

	ec := DetectExecutionContext()
	assert.Equal(t, NameCopilot, ec.Agent)
	assert.Equal(t, NameIterm, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotViaAIAgentAlias(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv(EnvAIAgent, AliasGitHubCopilotVSCodeAgent)

	ec := DetectExecutionContext()
	assert.Equal(t, NameCopilot, ec.Agent)
	assert.Equal(t, NameVSCode, ec.Client)
}

func TestDetectExecutionContext_ClientSkippedForHuman(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	// No agent signal: a human in a VS Code or Zed terminal must not be recorded,
	// however loudly the editor announces itself.
	t.Setenv("TERM_PROGRAM", NameVSCode)
	t.Setenv("ZED_TERM", "true")

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Client)
}

// The host axis must follow the editor, not the agent: the same agent reports a
// different window depending on where the user opened it.
func TestDetectExecutionContext_ClientFollowsHostEditor(t *testing.T) {
	askpass := func(app string) map[string]string {
		return map[string]string{EnvVSCodeGitAskpassMain: "/Applications/" + app + "/out/askpass-main.js"}
	}
	testCases := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{"copilot in jetbrains", map[string]string{EnvCopilotAgent: "1", "TERMINAL_EMULATOR": "JetBrains-JediTerm"}, NameJetBrains},
		{"claude in jetbrains", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", "TERMINAL_EMULATOR": "JetBrains-JediTerm"}, NameJetBrains},
		{"claude in zed", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", "ZED_TERM": "true"}, NameZed},
		{"claude in cursor", mergeEnv(map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1"}, askpass("Cursor.app")), NameCursor},
		{"cline in windsurf", mergeEnv(map[string]string{"CLINE_ACTIVE": "1"}, askpass("Windsurf.app")), NameWindsurf},
		{"gemini in antigravity", mergeEnv(map[string]string{"GEMINI_CLI": "1"}, askpass("Antigravity.app")), NameAntigravity},
		// A fork inherits the upstream Copilot marker; the fork must still win.
		{"copilot in windsurf", mergeEnv(map[string]string{EnvCopilotAgent: "1"}, askpass("Windsurf.app")), NameWindsurf},
		// Cursor keeps its identity when the window is proven by trace ID alone.
		{"claude in cursor via trace id", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", EnvCursorTraceID: "abc"}, NameCursor},
		// Claude Code is itself the app when no IDE is proven.
		{"claude in a plain terminal", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", "TERM_PROGRAM": "iTerm.app"}, NameClaude},
		{"claude in vscode via askpass", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", EnvVSCodeGitAskpassMain: "/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist/askpass-main.js"}, NameVSCode},
		{"windsurf agent without askpass", map[string]string{"WINDSURF_CASCADE_TERMINAL": "1", "TERM_PROGRAM": NameVSCode}, NameWindsurf},
		{"antigravity agent without askpass", map[string]string{"ANTIGRAVITY_AGENT": "1", "TERM_PROGRAM": NameVSCode}, NameAntigravity},
		// Other CLI agents fall back to the terminal app.
		{"gemini in iterm", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": "iTerm.app"}, NameIterm},
		{"gemini in warp", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": "WarpTerminal"}, NameWarp},
		{"gemini in apple terminal", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": "Apple_Terminal"}, NameTerminal},
		{"gemini in tmux", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": NameTmux}, NameTmux},
		{"generic git askpass does not claim cursor", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": "iTerm.app", "GIT_ASKPASS": "/Users/cursor/bin/git-helper"}, NameIterm}, // #nosec G101 -- fixture path, not a credential
		// A VS Code install under a login named "cursor" must not become client=cursor.
		{"vscode askpass under cursor home is vscode", map[string]string{"GEMINI_CLI": "1", EnvVSCodeGitAskpassMain: "/Users/cursor/AppData/Local/Programs/Microsoft VS Code/resources/app/extensions/git/dist/askpass-main.js"}, NameVSCode},
		{"windows cursor install is cursor", map[string]string{"GEMINI_CLI": "1", EnvVSCodeGitAskpassMain: `C:\Users\bob\AppData\Local\Programs\cursor\resources\app\extensions\git\dist\askpass-main.js`}, NameCursor},
		// Inherited TERM_PROGRAM=vscode is not a vscode window (P13). Askpass of
		// stock VS Code is stronger and does prove the host.
		{"gemini with inherited vscode term", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": NameVSCode}, ""},
		{"copilot cli with inherited vscode term", map[string]string{"COPILOT_CLI": "1", "TERM_PROGRAM": NameVSCode}, ""},
		{"copilot cli in stock vscode via askpass", map[string]string{"COPILOT_CLI": "1", EnvVSCodeGitAskpassMain: "/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist/askpass-main.js"}, NameVSCode},
		{"gemini in unknown terminal app", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": "FooBar.app"}, "foobar"},
		{"gemini in vscodium via askpass", mergeEnv(map[string]string{"GEMINI_CLI": "1"}, askpass("VSCodium.app")), NameCodium},
		{"trae agent is not vscode", map[string]string{"TRAE_AI_SHELL_ID": "1", "TERM_PROGRAM": NameVSCode}, NameTrae},
		{"copilot in visual studio", map[string]string{EnvCopilotAgent: "1", EnvVisualStudioVersion: "17.0"}, NameVisualStudio},
		{"gemini in tmux via TMUX", map[string]string{"GEMINI_CLI": "1", "TMUX": "1"}, NameTmux},
		{"gemini in windows terminal", map[string]string{"GEMINI_CLI": "1", "WT_SESSION": "abc"}, NameWindowsTerminal},
		{"gemini in ghostty via TERM", map[string]string{"GEMINI_CLI": "1", "TERM": "xterm-ghostty"}, NameGhostty},
		{"gemini in kitty via window id", map[string]string{"GEMINI_CLI": "1", "KITTY_WINDOW_ID": "1"}, NameKitty},
		{"gemini in alacritty via log", map[string]string{"GEMINI_CLI": "1", "ALACRITTY_LOG": "/tmp/alacritty.log"}, NameAlacritty},
		{"inherited vscode term plus tmux is tmux", map[string]string{"GEMINI_CLI": "1", "TERM_PROGRAM": NameVSCode, "TMUX": "1"}, NameTmux},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resetExecutionContextForTest(t)
			clearAgentEnvVars(t)
			for key, value := range testCase.env {
				t.Setenv(key, value)
			}

			ec := DetectExecutionContext()
			assert.True(t, ec.IsAgent)
			assert.Equal(t, testCase.expected, ec.Client)
		})
	}
}

func mergeEnv(envs ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, env := range envs {
		for key, value := range env {
			merged[key] = value
		}
	}
	return merged
}
