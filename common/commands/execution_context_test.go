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
	}
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
	t.Setenv("CLAUDECODE", "1")
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
	t.Setenv("CLAUDECODE", "1")

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
	t.Setenv("CLAUDECODE", "1")
	first := DetectExecutionContext()

	// Mutate env after first call; result must not change without reset.
	t.Setenv("CLAUDECODE", "")
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
	}
	t.Setenv("AGENT", "")
	t.Setenv("AI_AGENT", "")
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("JFROG_CLI_AI_MODEL", "")
}

func TestDetectExecutionContext_ModelAgentOnly(t *testing.T) {
	resetExecutionContextForTest(t)
	clearAgentEnvVars(t)
	t.Setenv("CLAUDECODE", "1")
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
	t.Setenv("CLAUDECODE", "1")
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
