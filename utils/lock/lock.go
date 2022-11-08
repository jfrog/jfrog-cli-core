package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Lock struct {
	// The current time when the lock was created
	currentTime int64
	// The full path to the lock file.
	fileName string
	pid      int
}

type Locks []Lock

func (locks Locks) Len() int {
	return len(locks)
}

func (locks Locks) Swap(i, j int) {
	locks[i], locks[j] = locks[j], locks[i]
}

func (locks Locks) Less(i, j int) bool {
	return locks[i].currentTime < locks[j].currentTime
}

// Creating a new lock object.
func (lock *Lock) createNewLockFile(lockDirPath string) error {
	lock.currentTime = time.Now().UnixNano()
	err := fileutils.CreateDirIfNotExist(lockDirPath)
	if err != nil {
		return err
	}
	lock.pid = os.Getpid()
	return lock.createFile(lockDirPath)
}

func (lock *Lock) getLockFilename(folderName string) string {
	return filepath.Join(folderName, "jfrog-cli.conf.lck."+strconv.Itoa(lock.pid)+"."+strconv.FormatInt(lock.currentTime, 10))
}

func (lock *Lock) createFile(folderName string) error {
	// We are creating an empty file with the pid and current time part of the name
	lock.fileName = lock.getLockFilename(folderName)
	file, err := os.OpenFile(lock.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if err = file.Close(); err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

// Try to acquire a lock
func (lock *Lock) lock() error {
	filesList, err := lock.getListOfFiles()
	if err != nil {
		return err
	}
	i := 0
	// Trying 1200 times to acquire a lock
	for i <= 1200 {
		// If only one file, means that the process that is running is the one that created the file.
		// We can continue
		if len(filesList) == 1 {
			return nil
		}

		locks, err := getLocks(filesList)
		if err != nil {
			return err
		}
		// If the first timestamp in the sorted locks slice is equal to this timestamp
		// means that the lock can be acquired
		if locks[0].currentTime == lock.currentTime {
			// Edge case, if at the same time (by the nanoseconds) two different process created two files.
			// We are checking the PID to know which process can run.
			if locks[0].pid != lock.pid {
				err := lock.removeOtherLockOrWait(locks[0], &filesList)
				if err != nil {
					return err
				}
			} else {
				log.Debug("Lock has been acquired for", lock.fileName)
				return nil
			}
		} else {
			err := lock.removeOtherLockOrWait(locks[0], &filesList)
			if err != nil {
				return err
			}
		}
		i++
	}
	return errors.New("lock hasn't been acquired")
}

// Checks if other lock file still exists.
// Or the process that created the lock still running.
func (lock *Lock) removeOtherLockOrWait(otherLock Lock, filesList *[]string) error {
	// Check if file exists.
	exists, err := fileutils.IsFileExists(otherLock.fileName, false)
	if err != nil {
		return err
	}

	if !exists {
		// Process already finished. Update the list.
		*filesList, err = lock.getListOfFiles()
		if err != nil {
			return err
		}
		return nil
	}
	log.Debug("Lock hasn't been acquired.")

	// Check if the process is running.
	// There are two implementation of the 'isProcessRunning'.
	// One for Windows and one for Unix based systems.
	running, err := isProcessRunning(otherLock.pid)
	if err != nil {
		return err
	}

	if !running {
		log.Debug(fmt.Sprintf("Removing lock file %s since the creating process is no longer running", otherLock.fileName))
		err := otherLock.Unlock()
		if err != nil {
			return err
		}
		// Update list of files
		*filesList, err = lock.getListOfFiles()
		return err
	}
	// Other process is running. Wait.
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (lock *Lock) getListOfFiles() ([]string, error) {
	// Listing all the files in the lock directory
	filesList, err := fileutils.ListFiles(filepath.Dir(lock.fileName), false)
	if err != nil {
		return nil, err
	}
	return filesList, nil
}

// Returns a list of all available locks.
func getLocks(filesList []string) (Locks, error) {
	// Slice of all the timestamps that currently the lock directory has
	var files Locks
	for _, path := range filesList {
		fileName := filepath.Base(path)
		splitted := strings.Split(fileName, ".")

		if len(splitted) != 5 {
			return nil, errorutils.CheckErrorf("Failed while parsing the file name: %s located at: %s. Expecting a different format.", fileName, path)
		}
		// Last element is the timestamp.
		time, err := strconv.ParseInt(splitted[4], 10, 64)
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		pid, err := strconv.Atoi(splitted[3])
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		file := Lock{
			currentTime: time,
			pid:         pid,
			fileName:    path,
		}
		files = append(files, file)
	}
	sort.Sort(files)
	return files, nil
}

// Removes the lock file so other process can continue.
func (lock *Lock) Unlock() error {
	log.Debug("Releasing lock: ", lock.fileName)
	exists, err := fileutils.IsFileExists(lock.fileName, false)
	if err != nil {
		return err
	}

	if exists {
		err = os.Remove(lock.fileName)
		if err != nil {
			return errorutils.CheckError(err)
		}
	}
	return nil
}

func CreateLock(lockDirPath string) (unlock func() error, err error) {
	log.Debug("Creating lock in: ", lockDirPath)
	lockFile := new(Lock)
	unlock = func() error { return lockFile.Unlock() }
	err = lockFile.createNewLockFile(lockDirPath)
	if err != nil {
		return
	}

	// Trying to acquire a lock for the running process.
	err = lockFile.lock()
	if err != nil {
		err = errorutils.CheckError(err)
	}
	return
}

func GetLastLockTimestamp(lockDirPath string) (int64, error) {
	filesList, err := fileutils.ListFiles(lockDirPath, false)
	if err != nil {
		return 0, err
	}
	if len(filesList) == 0 {
		return 0, nil
	}
	locks, err := getLocks(filesList)
	if err != nil || len(locks) == 0 {
		return 0, err
	}

	lastLock := locks[len(locks)-1]

	// If the lock isn't acquired by a running process, an unexpected error was occurred.
	running, err := isProcessRunning(lastLock.pid)
	if err != nil {
		return 0, err
	}
	if !running {
		return 0, nil
	}

	return lastLock.currentTime, nil
}
