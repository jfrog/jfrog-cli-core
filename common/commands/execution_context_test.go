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
	t.Setenv("CURSOR_TRACE_ID", "trace-123")
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgent_CopilotConfigNotASessionMarker(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("COPILOT_MODEL", "gpt-5.2")
	assert.Equal(t, "", detectAgent())
	t.Setenv("COPILOT_ALLOW_ALL", "true")
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

func TestDetectAgent_GenericAgentEnvCollapsesToUnknown(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("AGENT", "some_random_value")
	assert.Equal(t, AgentUnknown, detectAgent())
}

func TestDetectAgent_GenericValueMapsToKnownAgent(t *testing.T) {
	cases := map[string]string{
		"claude-code":     "claude",
		"gemini-cli":      "gemini",
		"github-copilot":  "copilot",
		"roo-code":        "roo_code",
		"roo_code":        "roo_code", // canonical form round-trips
		"qwen":            "qwen",
		"amazon-q-cli":    "amazon_q",
		"amazon_q":        "amazon_q", // alias-only id still round-trips
		"aider":           "aider",    // table row with no session env
		"goose@1.2.3":     "goose",    // version suffix stripped
		"CURSOR":          "cursor",   // case-insensitive
		"totally-made-up": AgentUnknown,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			clearAgentEnvVars(t)
			t.Setenv("AI_AGENT", value)
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
	assert.Equal(t, "gemini", detectAgent())
}

func TestDetectAgent_TableWinsOverGenericValue(t *testing.T) {
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("AI_AGENT", "cursor")
	assert.Equal(t, "claude", detectAgent())
}

func TestDetectAgent_None(t *testing.T) {
	clearAgentEnvVars(t)
	assert.Equal(t, "", detectAgent())
}

func TestDetectAgentTraceID(t *testing.T) {
	t.Setenv("CURSOR_TRACE_ID", "trace-abc")
	assert.Equal(t, "trace-abc", detectAgentTraceID("cursor"))
	// Trace ID gated on agent identity: a leaked CURSOR_TRACE_ID from an outer
	// shell must not be reused when the real invoker is a different agent.
	assert.Equal(t, "", detectAgentTraceID("claude"))
	assert.Equal(t, "", detectAgentTraceID(""))
}

func TestDetectExecutionContext_Agent(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")

	ec := DetectExecutionContext()
	assert.True(t, ec.IsAgent)
	assert.Equal(t, "claude", ec.Agent)
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
	assert.Equal(t, "claude", second.Agent)
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
	}
	t.Setenv("AGENT", "")
	t.Setenv("AI_AGENT", "")
	// Cleared even though no longer detectors — leftover process env must not
	// bleed into tests that assert human / strong-signal behaviour.
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("CURSOR_CLI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")
	t.Setenv("COPILOT_MODEL", "")
	t.Setenv("COPILOT_ALLOW_ALL", "")
	t.Setenv("KILO_IPC_SOCKET_PATH", "")
	t.Setenv("KILO_SERVER_PASSWORD", "")
	t.Setenv("ROO_CODE_IPC_SOCKET_PATH", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("JFROG_CLI_AI_MODEL", "")
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
	assert.Equal(t, "vscode", sanitizeToken("vscode"))
	assert.Equal(t, "apple_terminal", sanitizeToken("Apple_Terminal"))
	assert.Equal(t, "iterm.app", sanitizeToken("  iTerm.app  "))
	assert.Equal(t, "1.2.3-beta", sanitizeToken("1.2.3-beta"))
	// Header-splitting and stray characters (CR/LF, colon, spaces) are stripped.
	assert.Equal(t, "xyz", sanitizeToken("x\r\n y: z"))
	assert.Equal(t, "", sanitizeToken(""))
	// Pathological env values are truncated so they cannot inflate the wire payload.
	assert.Equal(t, maxTokenLen, len(sanitizeToken(strings.Repeat("a", maxTokenLen+100))))
}

func TestDetectExecutionContext_ClientAgentOnly(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("TERM_PROGRAM", "vscode")

	ec := DetectExecutionContext()
	assert.Equal(t, "vscode", ec.Client)
}

func TestDetectExecutionContext_ClientSkippedForHuman(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	// No agent signal: a human in a VS Code terminal must not be recorded.
	t.Setenv("TERM_PROGRAM", "vscode")

	ec := DetectExecutionContext()
	assert.False(t, ec.IsAgent)
	assert.Equal(t, "", ec.Client)
}
