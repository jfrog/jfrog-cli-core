package utils

import (
	"bytes"
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func WriteJsonResults(results []services.ScanResponse) (string, error) {
	out, err := fileutils.CreateTempFile()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	defer out.Close()
	bytesRes, err := json.Marshal(&results)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, bytesRes, "", "  ")
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	_, err = out.Write([]byte(content.String()))
	return out.Name(), errorutils.CheckError(err)
}
