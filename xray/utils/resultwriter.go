package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func WriteJsonResults(results *services.ScanResponse, dirPath string) error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	out, err := ioutil.TempFile(dirPath, timestamp+"-")
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer out.Close()
	bytesRes, err := json.Marshal(&results)
	if err != nil {
		return errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, bytesRes, "", "  ")
	if err != nil {
		return errorutils.CheckError(err)
	}
	_, err = out.Write([]byte(content.String()))
	return errorutils.CheckError(err)

}
