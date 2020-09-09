package log

import (
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
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

func SetDefaultLogger() {
	log.SetLogger(log.NewLogger(GetCliLogLevel(), nil))
}
