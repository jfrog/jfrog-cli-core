package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	// DefaultThreads is the default number of threads working while transferring Artifactory's data
	DefaultThreads = 8
	// Maximum working threads allowed to execute the AQL queries and upload chunks for build-info repositories
	MaxBuildInfoThreads = 8
	// Maximum working threads allowed to execute the AQL queries
	MaxChunkBuilderThreads = 16

	transferSettingsFile     = "transfer.conf"
	transferSettingsLockFile = "transfer-settings"
)

type TransferSettings struct {
	ThreadsNumber int `json:"threadsNumber,omitempty"`
}

func (ts *TransferSettings) CalcNumberOfThreads(buildInfoRepo bool) (chunkBuilderThreads, chunkUploaderThreads int) {
	chunkBuilderThreads = ts.ThreadsNumber
	chunkUploaderThreads = ts.ThreadsNumber
	if buildInfoRepo && MaxBuildInfoThreads < ts.ThreadsNumber {
		chunkBuilderThreads = MaxBuildInfoThreads
		chunkUploaderThreads = MaxBuildInfoThreads
	}
	if MaxChunkBuilderThreads < chunkBuilderThreads {
		chunkBuilderThreads = MaxChunkBuilderThreads
	}
	return
}

func LoadTransferSettings() (settings *TransferSettings, err error) {
	filePath, err := getSettingsFilePath()
	if err != nil {
		return
	}

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return nil, err
	}
	unlockFunc, err := lock.CreateLock(filepath.Join(locksDirPath, transferSettingsLockFile))
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		err = errors.Join(err, unlockFunc())
	}()
	if err != nil {
		return
	}

	exists, err := fileutils.IsFileExists(filePath, false)
	if err != nil || !exists {
		return
	}
	content, err := fileutils.ReadFile(filePath)
	if err != nil {
		return
	}
	err = errorutils.CheckError(json.Unmarshal(content, &settings))
	return
}

func SaveTransferSettings(settings *TransferSettings) (err error) {
	b, err := json.Marshal(&settings)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	var contentBuffer bytes.Buffer
	err = errorutils.CheckError(json.Indent(&contentBuffer, b, "", "  "))
	if err != nil {
		return
	}
	bytesContent := contentBuffer.Bytes()
	filePath, err := getSettingsFilePath()
	if err != nil {
		return
	}

	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return
	}
	unlockFunc, err := lock.CreateLock(filepath.Join(locksDirPath, transferSettingsLockFile))
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		err = errors.Join(err, unlockFunc())
	}()
	if err != nil {
		return
	}

	err = errorutils.CheckError(os.WriteFile(filePath, bytesContent, 0600))
	return
}

func getSettingsFilePath() (string, error) {
	filePath, err := coreutils.GetJfrogTransferDir()
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
