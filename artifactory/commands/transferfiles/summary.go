package transferfiles

import (
	"encoding/json"
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
)

func GenerateSummaryFiles(logPaths []string) (err error) {
	allErrors, err := ReadErrorsFromLogFiles(logPaths)
	if err != nil {
		return
	}
	summaryCsv, err := os.Create("mySummary.csv")
	if err != nil {
		return
	}
	defer func() {
		e := summaryCsv.Close()
		if err == nil {
			err = e
		}
	}()
	return gocsv.MarshalFile(&allErrors, summaryCsv)
}

func ReadErrorsFromLogFiles(logPaths []string) (allErrors FilesErrors, err error) {
	for _, logPath := range logPaths {
		var exists bool
		exists, err = fileutils.IsFileExists(logPath, false)
		if err != nil {
			return
		}
		if exists {
			var content []byte
			content, err = fileutils.ReadFile(logPath)
			if err != nil {
				return
			}
			fileErrors := new(FilesErrors)
			err = errorutils.CheckError(json.Unmarshal(content, &fileErrors))
			if err != nil {
				return
			}
			allErrors.Errors = append(allErrors.Errors, fileErrors.Errors...)
		}
	}
	return
}
