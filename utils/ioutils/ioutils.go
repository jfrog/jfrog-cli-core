package ioutils

import (
	"bufio"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/term"
	"io"
	"os"
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
	return strings.ReplaceAll(filePath, "\\", "\\\\")
}

func UnixToWinPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "/", "\\\\")
}

func WinToUnixPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "\\", "/")
}
