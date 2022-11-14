package prechecks

import (
	"encoding/json"
	"fmt"
	"github.com/gocarina/gocsv"
	"github.com/gookit/color"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/artifactory"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"golang.org/x/exp/slices"
	"io"
	"sync"

	loguitils "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"time"
)

const (
	propertyAqlPaginationLimit = 1000000
	maxThreadCapacity          = 5000000
	threadCount                = 10
	maxAllowedValLength        = 2400
	retries                    = 10
	retriesWaitMilliSecs       = 1000
	longPropertyCheckName      = "Properties with value longer than 2.4K characters"
)

// Property - Represents a property of an item
type Property struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// valueLength - Equals to the value length
func (p *Property) valueLength() uint {
	return uint(len(p.Value))
}

// FileWithLongProperty - Represent a failed instance; File with property that has failed the check (used for csv report format)
type FileWithLongProperty struct {
	// The file that contains the long property
	api.FileRepresentation
	// The length of the property's value
	Length uint `json:"value-length,omitempty"`
	// The property that failed the check
	Property
}

type LongPropertyCheck struct{}

func NewLongPropertyCheck() *LongPropertyCheck {
	return &LongPropertyCheck{}
}
func (lpc *LongPropertyCheck) Name() string {
	return longPropertyCheckName
}

func (lpc *LongPropertyCheck) executeCheck(args RunArguments) (passed bool, err error) {
	var longProperties []Property
	var filesWithLongProperty []FileWithLongProperty
	// Get all the long property in the server
	if longProperties, err = lpc.getLongProperties(args); err != nil {
		return
	}
	log.Debug(fmt.Sprintf("Found %d properties with long value.", len(longProperties)))
	if len(longProperties) > 0 {
		// Search for files that contains them
		if filesWithLongProperty, err = searchPropertiesInFiles(longProperties, args); err != nil {
			return
		}
		if len(filesWithLongProperty) > 0 {
			err = handleFailureRun(filesWithLongProperty)
			return
		}
	}
	passed = true
	return
}

// Fetch all the properties from the server (with pagination) and keep only the properties with long values
func (lpc *LongPropertyCheck) getLongProperties(args RunArguments) (longProperties []Property, err error) {
	serviceManager, err := utils.CreateServiceManagerWithContext(args.context, args.serverDetails, false, 0, retries, retriesWaitMilliSecs)
	if err != nil {
		return
	}
	var query *AqlPropertySearchResult
	// Create progress display
	if args.progressMng != nil {
		propertyCount := args.progressMng.NewUpdatableHeadlineBarWithSpinner(func() string {
			return "Searching for long properties: " + color.Green.Render(len(longProperties))
		})
		defer propertyCount.Abort(true)
	}
	// Search
	pageCounter := 0
	for {
		if query, err = runSearchPropertyAql(serviceManager, pageCounter); err != nil {
			return
		}
		log.Debug(fmt.Sprintf("Found %d properties in the batch (isLastBatch=%t)", len(query.Results), len(query.Results) < propertyAqlPaginationLimit))
		for _, property := range query.Results {
			if long := isLongProperty(property); long {
				longProperties = append(longProperties, property)
				log.Debug(fmt.Sprintf(`Found long property '@%s':'%s')`, property.Key, property.Value))
			}
		}
		if len(query.Results) < propertyAqlPaginationLimit {
			break
		}
		pageCounter++
	}
	return
}

// Checks if the value of a property is not bigger than maxAllowedValLength
func isLongProperty(property Property) bool {
	return property.valueLength() > maxAllowedValLength
}

// AqlPropertySearchResult - the structure that returns from a property aql search
type AqlPropertySearchResult struct {
	Results []Property
}

// Get all the properties on the server using AQL (with pagination)
func runSearchPropertyAql(serviceManager artifactory.ArtifactoryServicesManager, pageNumber int) (result *AqlPropertySearchResult, err error) {
	result = &AqlPropertySearchResult{}
	err = runAqlService(serviceManager, getSearchAllPropertiesQuery(pageNumber), result)
	return
}

// Get the query that search properties on a server with pagination
func getSearchAllPropertiesQuery(pageNumber int) string {
	query := `properties.find()`
	query += fmt.Sprintf(`.sort({"$asc":["key"]}).offset(%d).limit(%d)`, pageNumber*propertyAqlPaginationLimit, propertyAqlPaginationLimit)
	return query
}

// SearchPropertiesInFiles Gets all the files that contains the given properties and are in the requested args.repos.
// Searching in multi threads one request for each property.
func searchPropertiesInFiles(properties []Property, args RunArguments) (files []FileWithLongProperty, err error) {
	// Initialize
	producerConsumer := parallel.NewRunner(threadCount, maxThreadCapacity, false)
	filesChan := make(chan FileWithLongProperty, threadCount)
	errorsQueue := clientutils.NewErrorsQueue(1)
	var progress *progressbar.TasksWithHeadlineProg
	if args.progressMng != nil {
		progress = args.progressMng.NewTasksWithHeadlineProg(int64(len(properties)), "Searching for files with the long property", false, progressbar.GREEN, "long property")
		defer args.progressMng.QuitTasksWithHeadlineProg(progress)
	}

	// Create routine to collect the files from the search tasks
	var waitCollection sync.WaitGroup
	waitCollection.Add(1)
	go func() {
		for current := range filesChan {
			files = append(files, current)
		}
		waitCollection.Done()
	}()
	// Create routines to search the property in the server
	go func() {
		defer producerConsumer.Done()
		for _, property := range properties {
			_, _ = producerConsumer.AddTaskWithError(createSearchPropertyTask(property, args, filesChan, progress), errorsQueue.AddError)
		}
	}()
	// Run
	producerConsumer.Run()
	close(filesChan)
	waitCollection.Wait()
	err = errorsQueue.GetError()
	return
}

// Create a task that fetch from the server the files with the given property.
// We keep only the files that are at the requested repos and pass them at the files channel
func createSearchPropertyTask(property Property, args RunArguments, filesChan chan FileWithLongProperty, progress *progressbar.TasksWithHeadlineProg) parallel.TaskFunc {
	return func(threadId int) (err error) {
		serviceManager, err := utils.CreateServiceManagerWithContext(args.context, args.serverDetails, false, 0, retries, retriesWaitMilliSecs)
		if err != nil {
			return
		}
		// Search
		var query *servicesUtils.AqlSearchResult
		if query, err = runSearchPropertyInFilesAql(serviceManager, property); err != nil {
			return
		}
		log.Debug(fmt.Sprintf("[Thread=%d] Got %d files from the query", threadId, len(query.Results)))
		for _, item := range query.Results {
			file := api.FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
			// Keep only if in the requested repos
			if slices.Contains(args.repos, file.Repo) {
				fileWithLongProperty := FileWithLongProperty{file, property.valueLength(), property}
				log.Debug(fmt.Sprintf("[Thread=%d] Found File{Repo=%s, Path=%s, Name=%s} with matching entry of long property.", threadId, file.Repo, file.Path, file.Name))
				filesChan <- fileWithLongProperty
			}
		}
		// Notify end of search for the current property
		if args.progressMng != nil && progress != nil {
			args.progressMng.Increment(progress)
		}
		return
	}
}

// Get all the files that contains the given property using AQL
func runSearchPropertyInFilesAql(serviceManager artifactory.ArtifactoryServicesManager, property Property) (result *servicesUtils.AqlSearchResult, err error) {
	result = &servicesUtils.AqlSearchResult{}
	err = runAqlService(serviceManager, getSearchPropertyInFilesQuery(property), result)
	return
}

// Get the query that search files with specific property
func getSearchPropertyInFilesQuery(property Property) string {
	return fmt.Sprintf(`items.find({"type": {"$eq":"file"},"@%s":{"$eq":"%s"}}).include("repo","path","name")`, property.Key, property.Value)
}

// Run AQL service that return a result in the given format structure 'v'
func runAqlService(serviceManager artifactory.ArtifactoryServicesManager, query string, v any) (err error) {
	reader, err := serviceManager.Aql(query)
	if err != nil {
		return
	}
	defer func() {
		if reader != nil {
			e := reader.Close()
			if err == nil {
				err = errorutils.CheckError(e)
			}
		}
	}()
	respBody, err := io.ReadAll(reader)
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = errorutils.CheckError(json.Unmarshal(respBody, v))
	return
}

// Create csv summary of all the files with long properties and log the result
func handleFailureRun(filesWithLongProperty []FileWithLongProperty) (err error) {
	// Create summary
	csvPath, err := createFailedCheckSummaryCsvFile(filesWithLongProperty, time.Now())
	if err != nil {
		log.Error("Couldn't create the long properties CSV file", err)
		return
	}
	// Log result
	nFails := len(filesWithLongProperty)
	propertyTxt := "entries"
	if nFails == 1 {
		propertyTxt = " entry"
	}
	log.Info(fmt.Sprintf("Found %d property %s with value longer than 2.4k characters. Check the summary CSV file in: %s", nFails, propertyTxt, csvPath))
	return
}

// Create a csv summary of all the files with long properties
func createFailedCheckSummaryCsvFile(failures []FileWithLongProperty, timeStarted time.Time) (csvPath string, err error) {
	// Create CSV file
	summaryCsv, err := loguitils.CreateCustomLogFile(fmt.Sprintf("long-properties-%s.csv", timeStarted.Format(loguitils.DefaultLogTimeLayout)))
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
	// Marshal JSON typed FileWithLongProperty array to CSV file
	err = errorutils.CheckError(gocsv.MarshalFile(failures, summaryCsv))
	return
}
