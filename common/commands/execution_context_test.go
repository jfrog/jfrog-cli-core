package commands

import (
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
	prevOnce, prevCache := executionContextOnce, cachedExecutionContext
	executionContextOnce = sync.Once{}
	cachedExecutionContext = ExecutionContext{}
	t.Cleanup(func() {
		executionContextOnce, cachedExecutionContext = prevOnce, prevCache
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
	t.Setenv("CURSOR_TRACE_ID", "")
}
