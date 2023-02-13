package lock

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

var testLockDirPath string

func init() {
	log.SetDefaultLogger()
	locksDirPath, err := coreutils.GetJfrogLocksDir()
	if err != nil {
		return
	}
	testLockDirPath = filepath.Join(locksDirPath, "test")
}

// The lock mechanism prefers earlier lock requests. If two locks requests have same time stamps, it'll take the one with the smaller PID first.
// Here we test the functionality of a real process with a real PID and a dummy process with MaxInt pid.
func TestLockSmallerPid(t *testing.T) {
	// First creating the first lock object with special pid number that doesn't exist.
	firstLock := getLock(math.MaxInt32, t)
	// Creating a second lock object with the running PID
	secondLock := getLock(os.Getpid(), t)

	// Confirming that only two locks are located in the lock directory
	files, err := fileutils.ListFiles(testLockDirPath, false)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	// Performing lock. This should work since the first lock PID is not running. The lock() will remove it.
	assert.NoError(t, secondLock.lock())

	// Unlocking to remove the lock file.
	assert.NoError(t, secondLock.Unlock())

	// If timestamp equals, secondLock.lock() is not expected to delete first lock's file, since os.Getpid() < math.MaxInt32.
	if firstLock.currentTime == secondLock.currentTime {
		assert.NoError(t, firstLock.Unlock())
	}

	// Confirming that no locks are located in the lock directory
	files, err = fileutils.ListFiles(testLockDirPath, false)
	assert.NoError(t, err)
	assert.Empty(t, files)
}

// The lock mechanism prefers earlier lock requests. If two locks requests have same time stamps, it'll take the one with the smaller PID first.
// Here we test the functionality of a real process with a real PID and a dummy process with -1 pid.
func TestLockBiggerPid(t *testing.T) {
	// First creating the first lock object with special pid number that doesn't exist.
	getLock(-1, t)
	// Creating a second lock object with the running PID
	secondLock := getLock(os.Getpid(), t)

	// Confirming that only two locks are located in the lock directory
	files, err := fileutils.ListFiles(testLockDirPath, false)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	// Performing lock. This should work since the first lock PID is not running. The lock() will remove it.
	assert.NoError(t, secondLock.lock())

	// Unlocking to remove the lock file.
	assert.NoError(t, secondLock.Unlock())

	// Confirming that no locks are located in the lock directory
	files, err = fileutils.ListFiles(testLockDirPath, false)
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestUnlock(t *testing.T) {
	lock := new(Lock)
	assert.NotZero(t, testLockDirPath, "An error occurred while initializing testLockDirPath")
	err := lock.createNewLockFile(testLockDirPath)
	assert.NoError(t, err)

	exists, err := fileutils.IsFileExists(lock.fileName, false)
	assert.NoError(t, err)
	assert.Truef(t, exists, "File %s is missing", lock.fileName)

	assert.NoError(t, lock.Unlock())
	exists, err = fileutils.IsFileExists(lock.fileName, false)
	assert.NoError(t, err)
	assert.Falsef(t, exists, "File %s exists, but it should have been removed by Unlock", lock.fileName)
}

func TestCreateFile(t *testing.T) {
	pid := os.Getpid()
	lock := getLock(pid, t)

	exists, err := fileutils.IsFileExists(lock.fileName, false)
	assert.NoError(t, err)
	assert.True(t, exists, "Lock wan't created.")

	files, err := fileutils.ListFiles(testLockDirPath, false)
	assert.NoError(t, err)
	assert.Lenf(t, files, 1, "Expected one file, got %d.", len(files))
	assert.Equalf(t, lock.fileName, files[0], "Expected filename %s, got %s", lock.fileName, files[0])

	// Removing the created lock file
	assert.NoError(t, lock.Unlock())
}

func TestGetLastLockTimestamp(t *testing.T) {
	// Create an empty dir and make sure we get zero timestamp
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	timestamp, err := GetLastLockTimestamp(tmpDir)
	assert.NoError(t, err)
	assert.Zero(t, timestamp)

	// Create a lock and make sure the timestamps are equal
	lock := getLock(os.Getpid(), t)
	timestamp, err = GetLastLockTimestamp(testLockDirPath)
	assert.NoError(t, err)
	assert.Equal(t, lock.currentTime, timestamp)

	// Removing the created lock file
	assert.NoError(t, lock.Unlock())
}

func TestGetLastLockNotRunningTimestamp(t *testing.T) {
	// Create an empty dir and make sure we get zero timestamp
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	timestamp, err := GetLastLockTimestamp(tmpDir)
	assert.NoError(t, err)
	assert.Zero(t, timestamp)

	// Create a lock for a non-running process and make sure the timestamp is zero
	lock := getLock(math.MaxInt-1, t)
	timestamp, err = GetLastLockTimestamp(testLockDirPath)
	assert.NoError(t, err)
	assert.Zero(t, timestamp)

	// Removing the created lock file
	assert.NoError(t, lock.Unlock())
}

func getLock(pid int, t *testing.T) Lock {
	lock := Lock{pid: pid, currentTime: time.Now().UnixNano()}
	assert.NotZero(t, testLockDirPath, "An error occurred while initializing testLockDirPath")
	assert.NoError(t, fileutils.CreateDirIfNotExist(testLockDirPath))
	assert.NoError(t, lock.createFile(testLockDirPath))
	return lock
}
