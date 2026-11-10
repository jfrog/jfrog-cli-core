//go:build !windows

package config

import (
	"errors"
	"os"
	"path/filepath"
)

func replaceCredentialConfig(source, destination string) (bool, error) {
	return replaceConfigWithSync(source, destination, syncConfigDirectory)
}

func replaceConfigWithSync(source, destination string, syncDir func(string) error) (bool, error) {
	if err := os.Rename(source, destination); err != nil {
		return false, err
	}
	return true, syncDir(filepath.Dir(destination))
}

func syncConfigDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
