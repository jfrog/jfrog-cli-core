package log

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	golangLog "log"
	"os"
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
