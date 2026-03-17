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

