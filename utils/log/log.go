package log

import "github.com/jfrog/jfrog-client-go/utils/log"

func SetDefaultLogger() {
	log.SetLogger(log.NewLogger(log.INFO, nil))
}
