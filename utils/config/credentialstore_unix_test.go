//go:build !windows

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentialConfigDirectorySyncFailureReportsReplacement(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(source, []byte("new config"), 0600))
	require.NoError(t, os.WriteFile(destination, []byte("old config"), 0600))
	called := false
	replaced, err := replaceConfigWithSync(source, destination, func(path string) error {
		called = true
		require.Equal(t, dir, path)
		data, readErr := os.ReadFile(destination)
		require.NoError(t, readErr)
		require.Equal(t, "new config", string(data))
		return errors.New("directory sync failed")
	})
	require.True(t, called)
	require.True(t, replaced)
	require.ErrorContains(t, err, "directory sync failed")
}
