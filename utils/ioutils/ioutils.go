package ioutils

import (
	"bufio"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	terminal "golang.org/x/term"
	"io"
	"os"
	"strings"
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

func ScanPasswordFromConsole(message string) (password string, err error) {
	fmt.Print(message)
	var fileDescriptor int
	stdin := 0
	if terminal.IsTerminal(stdin) {
		fileDescriptor = stdin
		inputPass, e := terminal.ReadPassword(fileDescriptor)
		if e != nil {
			return "", e
		}
		password = string(inputPass)
	} else {
		// Handling non-terminal sources.
		// When command is running from external script - reading input should be done using a buffer.
		reader := bufio.NewReader(os.Stdin)
		password, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}
	}
	return
}

func ScanFromConsole(caption string, scanInto *string, defaultValue string) {
	if defaultValue != "" {
		fmt.Print(caption + " [" + defaultValue + "]: ")
	} else {
		fmt.Print(caption + ": ")
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
