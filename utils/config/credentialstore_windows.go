package config

import "golang.org/x/sys/windows"

func replaceCredentialConfig(source, destination string) (bool, error) {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return false, err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return false, err
	}
	// Replace an existing file and request write-through completion. Windows
	// does not support the Unix directory-fsync approach.
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	return err == nil, err
}
