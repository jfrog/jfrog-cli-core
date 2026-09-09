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
	t.Setenv(envCursorTraceID, "trace-123")
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
	assert.Equal(t, nameClaude, detectAgent())
}

func TestDetectAgent_AmazonQUnrelatedExecutionEnv(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("AWS_EXECUTION_ENV", "AWS_ECS_FARGATE")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_AmazonQExecutionEnv(t *testing.T) {
	// Literal is independent of the detector table so a typo there fails this test.
	clearAgentEnvVars(t)
	t.Setenv("AWS_EXECUTION_ENV", "AmazonQ-For-CLI")
	assert.Equal(t, nameAmazonQ, detectAgent())
}

func TestJediTermMatchesJetBrainsOSValue(t *testing.T) {
	// JetBrains sets TERMINAL_EMULATOR=JetBrains-JediTerm (capital J).
	// Assert the literal so a const rename cannot hide a production miss.
	assert.Equal(t, "JetBrains-JediTerm", jediTerm)
}

func TestDetectAgent_GenericAgentEnvCollapsesToUnknown(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv(envAgent, "some_random_value")
	assert.Equal(t, AgentUnknown, detectAgent())
}

func TestDetectAgent_GenericValueMapsToKnownAgent(t *testing.T) {
	cases := map[string]string{
		"claude-code":                 nameClaude,
		"gemini-cli":                  nameGemini,
		"github-copilot":              nameCopilot,
		"roo-code":                    nameRooCode,
		nameRooCode:                   nameRooCode, // canonical form round-trips
		nameQwen:                      nameQwen,
		"amazon-q-cli":                nameAmazonQ,
		nameAmazonQ:                   nameAmazonQ,
		"aider":                       "aider",
		"goose@1.2.3":                 "goose",
		"CURSOR":                      nameCursor,
		aliasGitHubCopilotVSCodeAgent: nameCopilot,
		"grok-cli":                    nameGrok,
		"grok-build":                  nameGrok,
		"totally-made-up":             AgentUnknown,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			clearAgentEnvVars(t)
			t.Setenv(envAIAgent, value)
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
	assert.Equal(t, nameGemini, detectAgent())
}

func TestDetectAgent_TableWinsOverGenericValue(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv(envAIAgent, nameCursor)
	assert.Equal(t, nameClaude, detectAgent())
}

func TestDetectAgent_None(t *testing.T) {
	clearAgentEnvVars(t)
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgentTraceID(t *testing.T) {
	t.Setenv(envCursorTraceID, "trace-abc")
	assert.Equal(t, "trace-abc", detectAgentTraceID(nameCursor))
	// Trace ID gated on agent identity: a leaked CURSOR_TRACE_ID from an outer
	// shell must not be reused when the real invoker is a different agent.
	assert.Equal(t, "", detectAgentTraceID(nameClaude))
	assert.Equal(t, "", detectAgentTraceID(""))
}

func TestDetectExecutionContext_Agent(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")

	ec := DetectExecutionContext()
	assert.True(t, ec.IsAgent)
	assert.Equal(t, nameClaude, ec.Agent)
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
	assert.Equal(t, nameClaude, second.Agent)
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
	t.Setenv(envAgent, "")
	t.Setenv(envAIAgent, "")
	// Cleared even though no longer detectors — leftover process env must not
	// bleed into tests that assert human / strong-signal behaviour.
	t.Setenv(envCursorTraceID, "")
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
	t.Setenv(envTermProgram, "")
	t.Setenv(envTerm, "")
	t.Setenv(envTmux, "")
	t.Setenv(envWTSession, "")
	t.Setenv(envKittyWindowID, "")
	t.Setenv(envAlacrittyLog, "")
	t.Setenv(envJFrogCLIAIModel, "")
	t.Setenv(envCopilotAgent, "")
	t.Setenv("COPILOT_AGENT_JOB_ID", "")
	t.Setenv(envVisualStudioVersion, "")
	// Host-window signals: a developer running these tests from a Zed, JetBrains
	// or Cursor terminal must not have their editor leak into client assertions.
	t.Setenv(envZedTerm, "")
	t.Setenv(envTerminalEmulator, "")
	t.Setenv(envGitAskpass, "")
	for _, env := range askpassEnvVars {
		t.Setenv(env, "")
	}
}

func TestDetectExecutionContext_ModelAgentOnly(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv(envJFrogCLIAIModel, "opus-4.7")

	assert.Equal(t, "opus-4.7", DetectExecutionContext().Model)
}

func TestDetectExecutionContext_ModelSkippedForHuman(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv(envJFrogCLIAIModel, "opus-4.7")

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Model)
}

func TestSanitizeWireName(t *testing.T) {
	assert.Equal(t, nameVSCode, sanitizeWireName(nameVSCode))
	assert.Equal(t, "apple_terminal", sanitizeWireName(termProgramApple))
	assert.Equal(t, "iterm.app", sanitizeWireName("  "+termProgramItermApp+"  "))
	assert.Equal(t, "1.2.3-beta", sanitizeWireName("1.2.3-beta"))
	// Header-splitting and stray characters (CR/LF, colon, spaces) are stripped.
	assert.Equal(t, "xyz", sanitizeWireName("x\r\n y: z"))
	assert.Equal(t, "", sanitizeWireName(""))
	// Pathological env values are truncated so they cannot inflate the wire payload.
	assert.Equal(t, maxWireNameLen, len(sanitizeWireName(strings.Repeat("a", maxWireNameLen+100))))
}

func TestCanonicalTerminalName(t *testing.T) {
	assert.Equal(t, nameTerminal, canonicalTerminalName(termProgramApple))
	assert.Equal(t, nameIterm, canonicalTerminalName("  "+termProgramItermApp+"  "))
	assert.Equal(t, nameWarp, canonicalTerminalName(termProgramWarp))
	assert.Equal(t, nameWezterm, canonicalTerminalName("WezTerm"))
	assert.Equal(t, nameHyper, canonicalTerminalName("Hyper"))
	assert.Equal(t, "", canonicalTerminalName(nameVSCode))
	assert.Equal(t, "", canonicalTerminalName("FooBar.app"))
}

func TestDetectExecutionContext_ClientCursorIgnoresTermProgram(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CURSOR_AGENT", "1")
	t.Setenv(envCursorTraceID, "trace-123")
	t.Setenv(envTermProgram, nameVSCode)

	ec := DetectExecutionContext()
	assert.Equal(t, nameCursor, ec.Agent)
	assert.Equal(t, nameCursor, ec.Client)
}

func TestDetectExecutionContext_ClientClaudeAppWithoutKnownIDE(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv(envTermProgram, termProgramItermApp)

	ec := DetectExecutionContext()
	assert.Equal(t, nameClaude, ec.Agent)
	assert.Equal(t, nameClaude, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotVscodePlugin(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv(envCopilotAgent, "1")
	t.Setenv(envTermProgram, termProgramItermApp)

	ec := DetectExecutionContext()
	assert.Equal(t, nameCopilot, ec.Agent)
	assert.Equal(t, nameIterm, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotCLIFallsBackToTerminalApp(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("COPILOT_CLI", "1")
	t.Setenv(envTermProgram, termProgramItermApp)

	ec := DetectExecutionContext()
	assert.Equal(t, nameCopilot, ec.Agent)
	assert.Equal(t, nameIterm, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotViaAIAgentAlias(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv(envAIAgent, aliasGitHubCopilotVSCodeAgent)

	ec := DetectExecutionContext()
	assert.Equal(t, nameCopilot, ec.Agent)
	assert.Equal(t, nameVSCode, ec.Client)
}

func TestDetectExecutionContext_ClientCopilotAliasMatchesSession(t *testing.T) {
	testCases := []struct {
		name string
		key  string
		val  string
	}{
		{"AI_AGENT uppercase", envAIAgent, "GITHUB_COPILOT_VSCODE_AGENT"},
		{"AI_AGENT padded versioned", envAIAgent, "  github_copilot_vscode_agent@1.2.3  "},
		{"AGENT lowercase", envAgent, aliasGitHubCopilotVSCodeAgent},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resetExecutionContextForTest(t)
			clearAgentEnvVars(t)
			t.Setenv(testCase.key, testCase.val)

			ec := DetectExecutionContext()
			assert.Equal(t, nameCopilot, ec.Agent)
			assert.Equal(t, nameVSCode, ec.Client)
		})
	}
}

func TestDetectExecutionContext_ClientSkippedForHuman(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	// No agent signal: a human in a VS Code or Zed terminal must not be recorded,
	// however loudly the editor announces itself.
	t.Setenv(envTermProgram, nameVSCode)
	t.Setenv(envZedTerm, "true")

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Client)
}

// The host axis must follow the editor, not the agent: the same agent reports a
// different window depending on where the user opened it.
func TestDetectExecutionContext_ClientFollowsHostEditor(t *testing.T) {
	askpass := func(app string) map[string]string {
		return map[string]string{envVSCodeGitAskpassMain: vscodeGitHelperPath("/Applications/" + app + "/out")}
	}
	testCases := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{"copilot in jetbrains", map[string]string{envCopilotAgent: "1", envTerminalEmulator: "JetBrains-JediTerm"}, nameJetBrains},
		{"claude in jetbrains", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", envTerminalEmulator: "JetBrains-JediTerm"}, nameJetBrains},
		{"claude in zed", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", envZedTerm: "true"}, nameZed},
		{"claude in cursor", mergeEnv(map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1"}, askpass("Cursor.app")), nameCursor},
		{"cline in windsurf", mergeEnv(map[string]string{"CLINE_ACTIVE": "1"}, askpass("Windsurf.app")), nameWindsurf},
		{"gemini in antigravity", mergeEnv(map[string]string{"GEMINI_CLI": "1"}, askpass("Antigravity.app")), nameAntigravity},
		// A fork inherits the upstream Copilot marker; the fork must still win.
		{"copilot in windsurf", mergeEnv(map[string]string{envCopilotAgent: "1"}, askpass("Windsurf.app")), nameWindsurf},
		// Cursor keeps its identity when the window is proven by trace ID alone.
		{"claude in cursor via trace id", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", envCursorTraceID: "abc"}, nameCursor},
		// Claude Code is itself the app when no IDE is proven.
		{"claude in a plain terminal", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", envTermProgram: termProgramItermApp}, nameClaude},
		{"claude in vscode via askpass", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist")}, nameVSCode},
		{"windsurf agent without askpass", map[string]string{"WINDSURF_CASCADE_TERMINAL": "1", envTermProgram: nameVSCode}, nameWindsurf},
		{"antigravity agent without askpass", map[string]string{"ANTIGRAVITY_AGENT": "1", envTermProgram: nameVSCode}, nameAntigravity},
		// Other CLI agents fall back to the terminal app.
		{"gemini in iterm", map[string]string{"GEMINI_CLI": "1", envTermProgram: termProgramItermApp}, nameIterm},
		{"gemini in warp", map[string]string{"GEMINI_CLI": "1", envTermProgram: termProgramWarp}, nameWarp},
		{"gemini in apple terminal", map[string]string{"GEMINI_CLI": "1", envTermProgram: termProgramApple}, nameTerminal},
		{"gemini in tmux", map[string]string{"GEMINI_CLI": "1", envTermProgram: nameTmux}, nameTmux},
		{"generic git askpass does not claim cursor", map[string]string{"GEMINI_CLI": "1", envTermProgram: termProgramItermApp, envGitAskpass: "/Users/cursor/bin/git-helper"}, nameIterm}, // #nosec G101 jfrog-ignore -- fixture path, not a credential
		// A VS Code install under a login named "cursor" must not become client=cursor.
		{"vscode askpass under cursor home is vscode", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/Users/cursor/AppData/Local/Programs/Microsoft VS Code/resources/app/extensions/git/dist")}, nameVSCode},
		{"windows cursor install is cursor", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPathWin(`C:\Users\bob\AppData\Local\Programs\cursor\resources\app\extensions\git\dist`)}, nameCursor},
		{"cursor remote server askpass is cursor", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/home/ubuntu/.cursor-server/bin")}, nameCursor},
		{"windsurf remote server askpass is windsurf", map[string]string{"CLINE_ACTIVE": "1", envVSCodeGitAskpassNode: vscodeGitHelperPath("/home/ubuntu/.windsurf-server/bin")}, nameWindsurf},
		{"stock vscode remote server is vscode", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/home/ubuntu/.vscode-server/bin")}, nameVSCode},
		{"stock vscode via askpass node only", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassNode: vscodeGitHelperPath("/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist")}, nameVSCode},
		// Inherited TERM_PROGRAM=vscode is not a vscode window (P13). Askpass of
		// stock VS Code is stronger and does prove the host.
		{"gemini with inherited vscode term", map[string]string{"GEMINI_CLI": "1", envTermProgram: nameVSCode}, ""},
		{"copilot cli with inherited vscode term", map[string]string{"COPILOT_CLI": "1", envTermProgram: nameVSCode}, ""},
		{"copilot cli in stock vscode via askpass", map[string]string{"COPILOT_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist")}, nameVSCode},
		{"gemini in unknown terminal app", map[string]string{"GEMINI_CLI": "1", envTermProgram: "FooBar.app"}, ""},
		{"gemini in vscodium via askpass", mergeEnv(map[string]string{"GEMINI_CLI": "1"}, askpass("VSCodium.app")), nameCodium},
		{"trae agent is not vscode", map[string]string{"TRAE_AI_SHELL_ID": "1", envTermProgram: nameVSCode}, nameTrae},
		{"copilot in visual studio", map[string]string{envCopilotAgent: "1", envVisualStudioVersion: "17.0"}, nameVisualStudio},
		{"gemini in tmux via TMUX", map[string]string{"GEMINI_CLI": "1", envTmux: "1"}, nameTmux},
		{"gemini in windows terminal", map[string]string{"GEMINI_CLI": "1", envWTSession: "abc"}, nameWindowsTerminal},
		{"gemini in ghostty via TERM", map[string]string{"GEMINI_CLI": "1", envTerm: termXtermGhostty}, nameGhostty},
		{"gemini in kitty via window id", map[string]string{"GEMINI_CLI": "1", envKittyWindowID: "1"}, nameKitty},
		{"gemini in alacritty via log", map[string]string{"GEMINI_CLI": "1", envAlacrittyLog: "/tmp/alacritty.log"}, nameAlacritty},
		{"inherited vscode term plus tmux is tmux", map[string]string{"GEMINI_CLI": "1", envTermProgram: nameVSCode, envTmux: "1"}, nameTmux},
		{"cursor agent in stock vscode is vscode", map[string]string{"CURSOR_AGENT": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/git/dist")}, nameVSCode},
		{"cursor agent in iterm is iterm", map[string]string{"CURSOR_AGENT": "1", envTermProgram: termProgramItermApp}, nameIterm},
		{"cursor nightly askpass is not vscode", map[string]string{"GEMINI_CLI": "1", envVSCodeGitAskpassMain: vscodeGitHelperPath("/Applications/Cursor Nightly.app/Contents/Resources/app/extensions/git/dist")}, ""},
		{"copilot plugin with no terminal is vscode", map[string]string{envCopilotAgent: "1"}, nameVSCode},
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

// vscodeGitHelperPath builds a VS Code-family git-askpass fixture. The
// filename is isolated here so gosec G101 does not treat every table row
// as a hardcoded credential.
func vscodeGitHelperPath(dir string) string {
	return dir + "/askpass-main.js" // #nosec G101 jfrog-ignore -- fixture path, not a credential
}

func vscodeGitHelperPathWin(dir string) string {
	return dir + `\askpass-main.js` // #nosec G101 jfrog-ignore -- fixture path, not a credential
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
