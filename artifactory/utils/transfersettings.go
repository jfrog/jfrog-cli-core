package utils

import (
	"bytes"
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	transferSettingsFile     = "transfer.conf"
	transferSettingsLockFile = "transfer-settings"
)

type TransferSettings struct {
	ThreadsNumber int `json:"threadsNumber,omitempty"`
}

func LoadTransferSettings() (*TransferSettings, error) {
	filePath, err := getSettingsFilePath()
	if err != nil {
		return nil, err
	}

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return nil, err
	}
	lockFile, err := lock.CreateLock(filepath.Join(locksDirPath, transferSettingsLockFile))
	defer func() {
		e := lockFile.Unlock()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return nil, err
	}

	exists, err := fileutils.IsFileExists(filePath, false)
	if err != nil || !exists {
		return nil, err
	}
	content, err := fileutils.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	settings := new(TransferSettings)
	err = json.Unmarshal(content, &settings)
	return settings, err
}

func SaveTransferSettings(settings *TransferSettings) (err error) {
	b, err := json.Marshal(&settings)
	if err != nil {
		return errorutils.CheckError(err)
	}
	var contentBuffer bytes.Buffer
	err = json.Indent(&contentBuffer, b, "", "  ")
	if err != nil {
		return errorutils.CheckError(err)
	}
	bytesContent := contentBuffer.Bytes()
	filePath, err := getSettingsFilePath()
	if err != nil {
		return err
	}

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return err
	}
	lockFile, err := lock.CreateLock(filepath.Join(locksDirPath, transferSettingsLockFile))
	defer func() {
		e := lockFile.Unlock()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filePath, bytesContent, 0600)
	return errorutils.CheckError(err)
}

func getSettingsFilePath() (string, error) {
	filePath, err := coreutils.GetJfrogTransferFilesDir()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filePath, 0777)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	filePath = filepath.Join(filePath, transferSettingsFile)
	return filePath, nil
}
