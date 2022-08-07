package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"time"
)

// Create Errors Summary Csv File from given JSON log files
// logPaths    - Array of log files absolute paths.
// timeStarted - The time the command started, to include in the file name.
// csvPath     - Created CSV file path.
func createErrorsSummaryCsvFile(logPaths []string, timeStarted time.Time) (csvPath string, err error) {
	// Collect all errors from the given log files
	allErrors, err := parseErrorsFromLogFiles(logPaths)
	if err != nil {
		return
	}

	// Create errors CSV file
	summaryCsv, err := log.CreateCustomLogFile(fmt.Sprintf("transfer-files-logs-%s.csv", timeStarted.Format(log.DefaultLogTimeLayout)))
	if errorutils.CheckError(err) != nil {
		return
	}
	csvPath = summaryCsv.Name()
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

// Loop on json files containing FilesErrors and collect them to one FilesErrors object.
func parseErrorsFromLogFiles(logPaths []string) (allErrors FilesErrors, err error) {
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
