package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Create Errors Summary Csv File from given JSON log files
// logPaths   - array of log file absolute path's
// tmpDirPath - temp directory to store the CSV file
// csvPath    - Created CSV file path
func CreateErrorsSummaryCsvFile(logPaths []string, tmpDirPath string) (csvPath string, err error) {
	// Collect all errors from the given log files
	allErrors, err := ParseErrorsFromLogFiles(logPaths)
	if err != nil {
		return
	}

	// Create errors CSV file
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	// todo: create name convention and location for file
	csvPath = filepath.Join(tmpDirPath, fmt.Sprintf("logs-%s.csv", timestamp))
	summaryCsv, err := os.Create(csvPath)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}
	defer func() {
		e := summaryCsv.Close()
		if err == nil {
			err = e
		}
	}()
	// Marshal JSON typed FileUploadStatusResponse array to CSV file
	err = errorutils.CheckError(gocsv.MarshalFile(allErrors.Errors, summaryCsv))
	return
}

func ParseErrorsFromLogFiles(logPaths []string) (allErrors FilesErrors, err error) {
	for _, logPath := range logPaths {
		var exists bool
		exists, err = fileutils.IsFileExists(logPath, false)
		if err != nil {
			return
		}
		if !exists {
			err = fmt.Errorf("log file: %s does not exist", logPath)
			return
		}
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
	return
}
