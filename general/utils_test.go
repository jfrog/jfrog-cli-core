package general

import (
	"github.com/jfrog/jfrog-cli-core/v2/general/token"
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

func TestSetProviderTypeAsString_CaseInsensitive(t *testing.T) {
	cmd := token.NewOidcTokenExchangeCommand()
	cmd.ConfigOidcParams = &token.ConfigOidcParams{}

	testCases := []struct {
		input          string
		expectedOutput token.OidcProviderType
	}{
		{"github", token.GitHub},
		{"GITHUB", token.GitHub},
		{"GiThUb", token.GitHub},
		{"azure", token.Azure},
		{"Azure", token.Azure},
		{"genericoidc", token.GenericOidc},
	}

	for _, tc := range testCases {
		err := cmd.SetProviderTypeAsString(tc.input)
		assert.NoError(t, err)
		assert.Equal(t, tc.expectedOutput, cmd.ProviderType)
	}
}
