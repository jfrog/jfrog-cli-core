package lock

import (
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"syscall"
)

// This file will be compiled on Windows.
// Checks if the process can be reached.
// If an error occurs, check if the error is part of the invalid parameter. This means the process is not running.
// Else find the exit code. If the exit code 259 means the process is running.
func isProcessRunning(pid int) (bool, error) {
	process, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, true, uint32(pid))
	if err != nil {
		// Check if err is of type of syscall.Errno, which is a Windows error number.
		if errnoErr, ok := err.(syscall.Errno); ok {
			// 87 - error invalid parameter. For example during the tests when we provide a non existing PID
			if uintptr(errnoErr) == 87 {
				return false, nil
			}
		}
	}

	var exitCode uint32
	err = syscall.GetExitCodeProcess(process, &exitCode)
	if err != nil {
		return false, errorutils.CheckError(err)
	}

	// 259 - process still alive
	return exitCode == 259, nil
}
