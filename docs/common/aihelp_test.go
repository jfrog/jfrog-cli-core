package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// withAgentDetector installs an AIAgentDetector for the duration of a test.
// commands.DetectExecutionContext is sync.Once-memoized, so we can't reach
// the underlying detection by toggling env vars — instead we replace the
// hook AIHelpEnabled uses.
func withAgentDetector(t *testing.T, isAgent bool) {
	t.Helper()
	prev := AIAgentDetector
	AIAgentDetector = func() bool { return isAgent }
	t.Cleanup(func() { AIAgentDetector = prev })
}

func TestResolveDescription(t *testing.T) {
	const human = "human help"
	const ai = "ai help"

	tests := []struct {
		name      string
		envAIHelp string // pass "" to leave env unset
		setEnv    bool   // if false, don't touch the env var at all
		isAgent   bool
		ai        string
		expected  string
	}{
		{
			name:      "env force-on, no agent -> AI text",
			envAIHelp: "true",
			setEnv:    true,
			isAgent:   false,
			ai:        ai,
			expected:  ai,
		},
		{
			name:      "env force-off beats detected agent -> human text",
			envAIHelp: "false",
			setEnv:    true,
			isAgent:   true,
			ai:        ai,
			expected:  human,
		},
		{
			name:     "no env + agent detected -> AI text",
			setEnv:   false,
			isAgent:  true,
			ai:       ai,
			expected: ai,
		},
		{
			name:     "no env + no agent -> human text",
			setEnv:   false,
			isAgent:  false,
			ai:       ai,
			expected: human,
		},
		{
			name:     "agent detected + empty AI -> human fallback",
			setEnv:   false,
			isAgent:  true,
			ai:       "",
			expected: human,
		},
		{
			name:      "invalid env value falls back to detection (no agent here)",
			envAIHelp: "maybe",
			setEnv:    true,
			isAgent:   false,
			ai:        ai,
			expected:  human,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withAgentDetector(t, tc.isAgent)
			if tc.setEnv {
				t.Setenv(EnvAIHelp, tc.envAIHelp)
			}
			assert.Equal(t, tc.expected, ResolveDescription(human, tc.ai))
		})
	}
}

func TestAIHelpEnabledEnvParsing(t *testing.T) {
	// Detection deliberately returns true so we can prove env-parsing
	// short-circuits: only truthy/falsy env values should affect the result;
	// invalid values must fall through to the (here forced-true) detector.
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"TRUE", true},
		{"false", false},
		{"0", false},
		{"maybe", true}, // unparseable -> falls back to AIAgentDetector (true)
		{"", true},      // empty -> ParseBool error -> falls back to detector
	}
	for _, tc := range tests {
		t.Run("value="+tc.value, func(t *testing.T) {
			withAgentDetector(t, true)
			t.Setenv(EnvAIHelp, tc.value)
			assert.Equal(t, tc.expected, AIHelpEnabled())
		})
	}
}
