package general

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type deduceServerIdTest struct {
	url              string
	expectedServerID string
}

func TestDeduceServerId(t *testing.T) {
	testCases := []deduceServerIdTest{
		{"http://localhost:8082/", "localhost"},
		{"https://platform.jfrog.io/", "platform"},
		{"http://127.0.0.1:8082/", defaultServerId},
	}

	for _, testCase := range testCases {
		t.Run(testCase.url, func(t *testing.T) {
			serverId, err := deduceServerId(testCase.url)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedServerID, serverId)
		})
	}
}
