package golang

import (
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func LogGoVersion() error {
	output, err := goutils.GetGoVersion()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Using go:", output)
	return nil
}
