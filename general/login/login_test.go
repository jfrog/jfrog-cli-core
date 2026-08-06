package login

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	utilsTests "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetServerId(t *testing.T) {
	lc := NewLoginCommand()
	result := lc.SetServerId("my-server")
	assert.Equal(t, "my-server", lc.serverId)
	// Verify fluent API returns the same instance
	assert.Same(t, lc, result)
}

func TestSetDisableTokenRefresh(t *testing.T) {
	lc := NewLoginCommand()
	disableTokenRefresh := true
	result := lc.SetDisableTokenRefresh(&disableTokenRefresh)
	require.NotNil(t, lc.disableTokenRefresh)
	assert.True(t, *lc.disableTokenRefresh)
	// Verify fluent API returns the same instance
	assert.Same(t, lc, result)

	// Passing nil should leave any previously configured value untouched (i.e. clears back to "not specified")
	lc.SetDisableTokenRefresh(nil)
	assert.Nil(t, lc.disableTokenRefresh)
}

func TestRunWithNonExistentServerId(t *testing.T) {
	cleanUp, err := utilsTests.SetJfrogHome()
	require.NoError(t, err)
	defer cleanUp()

	// At least one server must exist so GetConfig actually looks up by ID
	err = config.SaveServersConf([]*config.ServerDetails{
		{ServerId: "other-server", Url: "https://example.jfrog.io/"},
	})
	require.NoError(t, err)

	lc := NewLoginCommand().SetServerId("non-existent-server")
	err = lc.Run()
	assert.ErrorContains(t, err, "non-existent-server")
}

