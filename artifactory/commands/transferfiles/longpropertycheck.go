package transferfiles

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils/precheckrunner"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/artifactory"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"golang.org/x/exp/slices"

	"github.com/jfrog/jfrog-client-go/utils/log"

	"time"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	propertyAqlPaginationLimit = 1000000
	maxThreadCapacity          = 5000000
	threadCount                = 10
	maxAllowedValLength        = 2400
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

type LongPropertyCheck struct {
	producerConsumer parallel.Runner
	filesChan        chan FileWithLongProperty
	errorsQueue      *clientutils.ErrorsQueue
	repos            []string
}

func NewLongPropertyCheck(repos []string) *LongPropertyCheck {
	return &LongPropertyCheck{repos: repos}
}

func (lpc *LongPropertyCheck) Name() string {
	return longPropertyCheckName
}

func (lpc *LongPropertyCheck) ExecuteCheck(args precheckrunner.RunArguments) (passed bool, err error) {
	// Init producer consumer
	lpc.producerConsumer = parallel.NewRunner(threadCount, maxThreadCapacity, false)
	lpc.filesChan = make(chan FileWithLongProperty, threadCount)
	lpc.errorsQueue = clientutils.NewErrorsQueue(1)
	var waitCollection sync.WaitGroup
	var filesWithLongProperty []FileWithLongProperty
	// Handle progress display
	var progress *progressbar.TasksProgressBar
	if args.ProgressMng != nil {
		progress = args.ProgressMng.NewTasksProgressBar(0, coreutils.IsWindows(), "long property")
		defer progress.GetBar().Abort(true)
	}
	// Create consumer routine to collect the files from the search tasks
	waitCollection.Add(1)
	go func() {
		for current := range lpc.filesChan {
			filesWithLongProperty = append(filesWithLongProperty, current)
		}
		waitCollection.Done()
	}()
	// Create producer routine to create search tasks for long properties in the server
	go func() {
		defer lpc.producerConsumer.Done()
		lpc.longPropertiesTaskProducer(progress, args)
	}()
	// Run
	lpc.producerConsumer.Run()
	close(lpc.filesChan)
	waitCollection.Wait()
	if err = lpc.errorsQueue.GetError(); err != nil {
		return
	}
	// Result
	if len(filesWithLongProperty) != 0 {
		err = handleFailureRun(filesWithLongProperty)
	} else {
		passed = true
	}
	return
}

// Search for long properties in the server and create a search task to find the files that contains them
// Returns the number of long properties found
func (lpc *LongPropertyCheck) longPropertiesTaskProducer(progress *progressbar.TasksProgressBar, args precheckrunner.RunArguments) int {
	// Init
	serviceManager, err := createTransferServiceManager(args.Context, args.ServerDetails)
	if err != nil {
		return 0
	}
	var propertyQuery *AqlPropertySearchResult
	longPropertiesCount := 0
	pageCounter := 0
	// Search
	for {
		if propertyQuery, err = runSearchPropertyAql(serviceManager, pageCounter); err != nil {
			return 0
		}
		log.Debug(fmt.Sprintf("Found %d properties in the batch (isLastBatch=%t)", len(propertyQuery.Results), len(propertyQuery.Results) < propertyAqlPaginationLimit))
		for _, property := range propertyQuery.Results {
			if long := isLongProperty(property); long {
				log.Debug(fmt.Sprintf(`Found long property ('@%s':'%s')`, property.Key, property.Value))
				if lpc.producerConsumer != nil {
					_, _ = lpc.producerConsumer.AddTaskWithError(createSearchPropertyTask(property, lpc.repos, args, lpc.filesChan, progress), lpc.errorsQueue.AddError)
				}
				if progress != nil {
					progress.IncGeneralProgressTotalBy(1)
				}
				longPropertiesCount++
			}
		}
		if len(propertyQuery.Results) < propertyAqlPaginationLimit {
			break
		}
		pageCounter++
	}

	return longPropertiesCount
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

// Create a task that fetch from the server the files with the given property.
// We keep only the files that are at the requested repos and pass them at the files channel
func createSearchPropertyTask(property Property, repos []string, args precheckrunner.RunArguments, filesChan chan FileWithLongProperty, progress *progressbar.TasksProgressBar) parallel.TaskFunc {
	return func(threadId int) (err error) {
		serviceManager, err := createTransferServiceManager(args.Context, args.ServerDetails)
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
			if slices.Contains(repos, file.Repo) {
				fileWithLongProperty := FileWithLongProperty{file, property.valueLength(), property}
				log.Debug(fmt.Sprintf("[Thread=%d] Found File{Repo=%s, Path=%s, Name=%s} with matching entry of long property.", threadId, file.Repo, file.Path, file.Name))
				filesChan <- fileWithLongProperty
			}
		}
		// Notify end of search for the current property
		if args.ProgressMng != nil && progress != nil {
			progress.GetBar().Increment()
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
	return fmt.Sprintf(`items.find({"type": {"$eq":"any"},"@%s":"%s"}).include("repo","path","name")`, property.Key, property.Value)
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
	csvPath, err := cmdutils.CreateCSVFile("long-properties", filesWithLongProperty, time.Now())
	if err != nil {
		log.Error("Couldn't create the long properties CSV file", err)
		return
	}
	// Log result
	nFails := len(filesWithLongProperty)
	propertyTxt := "entries"
	if nFails == 1 {
		propertyTxt = "entry"
	}
	log.Info(fmt.Sprintf("Found %d property %s with value longer than 2.4k characters. Check the summary CSV file in: %s", nFails, propertyTxt, csvPath))
	return
}
