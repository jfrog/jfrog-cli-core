package generic

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	clientartutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type SearchCommand struct {
	GenericCommand
}

func NewSearchCommand() *SearchCommand {
	return &SearchCommand{GenericCommand: *NewGenericCommand()}
}

func (sc *SearchCommand) CommandName() string {
	return "rt_search"
}

func (sc *SearchCommand) Run() error {
	reader, err := sc.Search()
	sc.Result().SetReader(reader)
	return err
}

func (sc *SearchCommand) Search() (*content.ContentReader, error) {
	// Service Manager
	serverDetails, err := sc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	servicesManager, err := utils.CreateServiceManager(serverDetails, sc.retries, sc.retryWaitTimeMilliSecs, false)
	if err != nil {
		return nil, err
	}
	// Search Loop
	log.Info("Searching artifacts...")

	searchResults, callbackFunc, err := utils.SearchFiles(servicesManager, sc.Spec())
	defer func() {
		err = errors.Join(err, callbackFunc())
	}()
	if err != nil {
		return nil, err
	}

	reader, err := utils.AqlResultToSearchResult(searchResults)
	if err != nil {
		return nil, err
	}
	length, err := reader.Length()
	clientartutils.LogSearchResults(length)
	return reader, err
}
