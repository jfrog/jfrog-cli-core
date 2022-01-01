package ioutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"golang.org/x/crypto/ssh/terminal"
)

// disallowUsingSavedPassword - Prevent changing username or url without changing the password.
// False iff the user changed the username or the url.
func ReadCredentialsFromConsole(details, savedDetails coreutils.Credentials, disallowUsingSavedPassword bool) error {
	if details.GetUser() == "" {
		tempUser := ""
		ScanFromConsole("JFrog username", &tempUser, savedDetails.GetUser())
		details.SetUser(tempUser)
		disallowUsingSavedPassword = true
	}
	if details.GetPassword() == "" {
		password, err := ScanPasswordFromConsole("JFrog password or API key: ")
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

func ScanPasswordFromConsole(message string) (string, error) {
	print(message)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	// New-line required after the password input:
	fmt.Println()
	return string(bytePassword), nil
}

func ScanFromConsole(caption string, scanInto *string, defaultValue string) {
	if defaultValue != "" {
		print(caption + " [" + defaultValue + "]: ")
	} else {
		print(caption + ": ")
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	*scanInto = scanner.Text()
	if *scanInto == "" {
		*scanInto = defaultValue
	}
	*scanInto = strings.TrimSpace(*scanInto)
}

func CopyFile(src, dst string, fileMode os.FileMode) (err error) {
	from, err := os.Open(src)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := from.Close()
		if err == nil {
			err = e
		}
	}()

	to, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, fileMode)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := to.Close()
		if err == nil {
			err = e
		}
	}()

	if _, err = io.Copy(to, from); err != nil {
		return errorutils.CheckError(err)
	}

	return errorutils.CheckError(os.Chmod(dst, fileMode))
}

func DoubleWinPathSeparator(filePath string) string {
	return strings.Replace(filePath, "\\", "\\\\", -1)
}

func UnixToWinPathSeparator(filePath string) string {
	return strings.Replace(filePath, "/", "\\\\", -1)
}

func WinToUnixPathSeparator(filePath string) string {
	return strings.Replace(filePath, "\\", "/", -1)
}
