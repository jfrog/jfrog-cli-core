package ioutils

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// disallowUsingSavedPassword - Prevent changing username or url without changing the password.
// False if the user changed the username or the url.
func ReadCredentialsFromConsole(details, savedDetails coreutils.Credentials, disallowUsingSavedPassword bool) error {
	if details.GetUser() == "" {
		tempUser := ""
		ScanFromConsole("JFrog username", &tempUser, savedDetails.GetUser())
		details.SetUser(tempUser)
		disallowUsingSavedPassword = true
	}
	if details.GetPassword() == "" {
		password, err := ScanJFrogPasswordFromConsole()
		if err != nil {
			return err
		}
		details.SetPassword(password)
		if details.GetPassword() == "" && !disallowUsingSavedPassword {
			details.SetPassword(savedDetails.GetPassword())
		}
	}

	return nil
}

func ScanJFrogPasswordFromConsole() (string, error) {
	return ScanPasswordFromConsole("JFrog password or API key: ")
}

func ScanPasswordFromConsole(message string) (string, error) {
	fmt.Print(coreutils.PrintLink(message))
	bytePassword, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	// New-line required after the password input:
	log.Output()
	return string(bytePassword), nil
}

func ScanFromConsole(caption string, scanInto *string, defaultValue string) {
	caption = coreutils.PrintLink(caption)
	if defaultValue != "" {
		caption = caption + " [" + defaultValue + "]"
	}
	fmt.Print(caption + ": ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	*scanInto = scanner.Text()
	if *scanInto == "" {
		*scanInto = defaultValue
	}
	*scanInto = strings.TrimSpace(*scanInto)
}

func DoubleWinPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "\\", "\\\\")
}

func UnixToWinPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "/", "\\\\")
}

func WinToUnixPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "\\", "/")
}

// BackupFile creates a backup of the file in filePath. The backup will be found at backupPath.
// The returned restore function can be called to restore the file's state - the file in filePath will be replaced by the backup in backupPath.
// If there is no file at filePath, a backup file won't be created, and the restore function will delete the file at filePath.
func BackupFile(filePath, backupFileName string) (restore func() error, err error) {
	fileInfo, err := os.Stat(filePath)
	if errorutils.CheckError(err) != nil {
		if os.IsNotExist(err) {
			restore = createRestoreFileFunc(filePath, backupFileName)
			err = nil
		}
		return
	}

	if err = cloneFile(filePath, backupFileName, fileInfo.Mode()); err != nil {
		return
	}
	log.Debug("The file", filePath, "was backed up successfully to", backupFileName)
	restore = createRestoreFileFunc(filePath, backupFileName)
	return
}

func cloneFile(origFile, newName string, fileMode os.FileMode) (err error) {
	from, err := os.Open(origFile)
	if errorutils.CheckError(err) != nil {
		return
	}
	defer func() {
		err = errors.Join(err, from.Close())
	}()

	to, err := os.OpenFile(filepath.Join(filepath.Dir(origFile), newName), os.O_RDWR|os.O_CREATE, fileMode)
	if errorutils.CheckError(err) != nil {
		return
	}
	defer func() {
		err = errors.Join(err, to.Close())
	}()

	if _, err = io.Copy(to, from); err != nil {
		err = errorutils.CheckError(err)
	}
	return
}

// createRestoreFileFunc creates a function for restoring a file from its backup.
// The returned function replaces the file in filePath with the backup in backupPath.
// If there is no file at backupPath (which means there was no file at filePath when BackupFile() was called), then the function deletes the file at filePath.
func createRestoreFileFunc(filePath, backupFileName string) func() error {
	return func() error {
		backupPath := filepath.Join(filepath.Dir(filePath), backupFileName)
		if _, err := os.Stat(backupPath); err != nil {
			if os.IsNotExist(err) {
				// We verify the existence of the file in the specified filePath before initiating its deletion in order to prevent errors that might occur when attempting to remove a non-existent file
				var fileExists bool
				fileExists, err = fileutils.IsFileExists(filePath, false)
				if err != nil {
					err = fmt.Errorf("failed to check for the existence of '%s' before deleting the file: %s", filePath, err.Error())
					return errorutils.CheckError(err)
				}
				if fileExists {
					err = os.Remove(filePath)
				}
				return errorutils.CheckError(err)
			}
			return errorutils.CheckErrorf(createRestoreErrorPrefix(filePath, backupPath) + err.Error())
		}

		if err := fileutils.MoveFile(backupPath, filePath); err != nil {
			return errorutils.CheckError(err)
		}
		log.Debug("Restored the file", filePath, "successfully")
		return nil
	}
}

func createRestoreErrorPrefix(filePath, backupPath string) string {
	return fmt.Sprintf("An error occurred while restoring the file: %s\n"+
		"To restore the file manually: delete %s and rename the backup file at %s (if exists) to '%s'.\n"+
		"Failure cause: ",
		filePath, filePath, backupPath, filePath)
}
