package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/require"
)

// Opt-in: this test accesses the real OS credential store and may prompt.
// On Linux, use a disposable D-Bus session and unlocked Secret Service.
func TestCredentialStoreNative(t *testing.T) {
	if os.Getenv("JFROG_CLI_TEST_KEYCHAIN") != "true" {
		t.Skip("set JFROG_CLI_TEST_KEYCHAIN=true to test a real OS keychain")
	}
	t.Setenv(coreutils.HomeDir, t.TempDir())
	t.Setenv(coreutils.EncryptionKey, "")
	t.Cleanup(func() {
		content, err := getConfigFile()
		if err != nil {
			t.Error(err)
			return
		}
		var conf Config
		if len(content) == 0 {
			return
		}
		if err = json.Unmarshal(content, &conf); err != nil {
			t.Error(err)
			return
		}
		service, err := credentialService()
		if err != nil {
			t.Error(err)
			return
		}
		if err = newCredentialStore().deleteCredentialRefs(service, conf.Servers); err != nil {
			t.Error(err)
		}
	})
	s := testCredentialServer()
	// Exercise chunked storage, including tokens exceeding Windows' blob limit.
	s.AccessToken = strings.Repeat("native-test-token-", 300)
	require.NoError(t, SaveServersConf([]*ServerDetails{s}))
	require.NoError(t, SetCredentialStore(CredentialStoreKeychain))
	loaded, err := GetSpecificConfig(s.ServerId, false, false)
	require.NoError(t, err)
	require.Equal(t, secretsOf(s), secretsOf(loaded))
	require.NotContains(t, string(configBytes(t)), s.AccessToken)
	require.NoError(t, withConfigLock(func() error {
		return writeNewTokens(loaded, s.ServerId, "native-updated-token", "native-updated-refresh", AccessToken)
	}))
	updated, err := GetSpecificConfig(s.ServerId, false, false)
	require.NoError(t, err)
	require.Equal(t, "native-updated-token", updated.AccessToken)
	require.NoError(t, SetCredentialStore(CredentialStoreFile))
	fileConfig, err := GetSpecificConfig(s.ServerId, false, false)
	require.NoError(t, err)
	require.Equal(t, secretsOf(updated), secretsOf(fileConfig))
}
