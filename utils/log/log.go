package log

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	golangLog "log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func GetCliLogLevel() log.LevelType {
	switch os.Getenv(coreutils.LogLevel) {
	case "ERROR":
		return log.ERROR
	case "WARN":
		return log.WARN
	case "DEBUG":
		return log.DEBUG
	default:
		return log.INFO
	}
}

func getJfrogCliLogTimestamp() int {
	switch os.Getenv(coreutils.LogTimestamp) {
	case "DATE_AND_TIME":
		return golangLog.Ldate | golangLog.Ltime | golangLog.Lmsgprefix
	case "OFF":
		return 0
	default:
		return golangLog.Ltime | golangLog.Lmsgprefix
	}
}

func SetDefaultLogger() {
	log.SetLogger(log.NewLoggerWithFlags(GetCliLogLevel(), nil, getJfrogCliLogTimestamp()))
}

func CreateLogFile() (*os.File, error) {
	logDir, err := coreutils.CreateDirInJfrogHome(coreutils.JfrogLogsDirName)
	if err != nil {
		return nil, err
	}

	currentTime := time.Now().Format("2006-01-02.15-04-05")
	pid := os.Getpid()

	fileName := filepath.Join(logDir, "jfrog-cli."+currentTime+"."+strconv.Itoa(pid)+".log")
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	return file, nil
}

// Closes the log file and resets to the default logger
func CloseLogFile(logFile *os.File) error {
	if logFile != nil {
		SetDefaultLogger()
		err := logFile.Close()
		return utils.CheckErrorWithMessage(err, "Failed closing the log file")
	}
	return nil
}
