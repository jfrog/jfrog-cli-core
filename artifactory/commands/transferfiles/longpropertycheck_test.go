package transferfiles

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils/precheckrunner"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/stretchr/testify/assert"
)

var (
	property        = Property{Key: "test.key", Value: "value"}
	shorterProperty = Property{Key: "shorter.key", Value: strings.Repeat("a", maxAllowedValLength-1)}
	borderProperty  = Property{Key: "border.key", Value: strings.Repeat("b", maxAllowedValLength)}
	longProperty    = Property{Key: "too.long.key", Value: strings.Repeat("c", maxAllowedValLength+1)}
	longProperty2   = Property{Key: "too.long.2.key", Value: strings.Repeat("dd", maxAllowedValLength)}

	file1 = api.FileRepresentation{Repo: "Repo", Path: "Path", Name: "Name"}
	file2 = api.FileRepresentation{Repo: "OtherRepo", Path: "Path", Name: "Name"}

	propertyToFiles = map[Property][]api.FileRepresentation{
		property:        {},
		shorterProperty: {file1},
		borderProperty:  {file2},
		longProperty:    {file1, file2},
		longProperty2:   {file1},
	}
)

func TestProperty(t *testing.T) {
	cases := []struct {
		property Property
		isLong   bool
	}{
		{Property{}, false},
		{shorterProperty, false},
		{borderProperty, false},
		{longProperty, true},
	}

	for _, testCase := range cases {
		testProperty(t, testCase.property, testCase.isLong)
	}
}

func testProperty(t *testing.T, property Property, isLong bool) {
	assert.Len(t, property.Value, int(property.valueLength()))
	long := isLongProperty(property)
	assert.Equal(t, isLong, long)
}

func TestGetLongProperties(t *testing.T) {
	cases := []struct {
		serverProperties []Property
		longProperties   []Property
	}{
		{[]Property{}, []Property{}},
		{[]Property{property, shorterProperty}, []Property{}},
		{[]Property{property, shorterProperty, longProperty2, borderProperty, longProperty}, []Property{longProperty, longProperty2}},
	}

	for _, testCase := range cases {
		testGetLongProperties(t, testCase.serverProperties, testCase.longProperties)
	}
}

func testGetLongProperties(t *testing.T, serverProperties, expectedLongProperties []Property) {
	testServer, serverDetails, _ := getLongPropertyCheckStubServer(t, serverProperties, propertyToFiles, false)
	defer testServer.Close()

	longPropertyCheck := NewLongPropertyCheck([]string{}, true)
	longPropertyCheck.filesChan = make(chan FileWithLongProperty, threadCount)

	count := longPropertyCheck.longPropertiesTaskProducer(nil, precheckrunner.RunArguments{Context: nil, ServerDetails: serverDetails})
	assert.Len(t, expectedLongProperties, count)
}

func TestSearchPropertyInFilesTask(t *testing.T) {
	cases := []struct {
		prop          Property
		specificRepos []string
		expected      []FileWithLongProperty
	}{
		{property, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{}},
		{borderProperty, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{{file2, borderProperty.valueLength(), borderProperty}}},
		{longProperty, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{{file1, longProperty.valueLength(), longProperty}, {file2, longProperty.valueLength(), longProperty}}},
		{longProperty, []string{"Repo"}, []FileWithLongProperty{{file1, longProperty.valueLength(), longProperty}}},
	}

	for _, testCase := range cases {
		testSearchPropertyInFilesTask(t, testCase.prop, testCase.specificRepos, propertyToFiles, testCase.expected)
	}
}

func testSearchPropertyInFilesTask(t *testing.T, prop Property, specificRepos []string, propertiesFiles map[Property][]api.FileRepresentation, expected []FileWithLongProperty) {
	testServer, serverDetails, _ := getLongPropertyCheckStubServer(t, nil, propertiesFiles, false)
	defer testServer.Close()

	var result []FileWithLongProperty
	filesChan := make(chan FileWithLongProperty)

	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		for current := range filesChan {
			result = append(result, current)
		}
		wait.Done()
	}()

	longPropertyCheck := LongPropertyCheck{
		filesChan: filesChan,
		repos:     specificRepos,
	}
	task := longPropertyCheck.createSearchPropertyTask(prop, precheckrunner.RunArguments{Context: nil, ServerDetails: serverDetails}, nil)
	assert.NoError(t, task(0))
	close(filesChan)
	wait.Wait()
	assert.ElementsMatch(t, expected, result)
}

func TestSearchLongPropertiesInFiles(t *testing.T) {
	cases := []struct {
		properties    []Property
		specificRepos []string
		expected      []FileWithLongProperty
	}{
		{[]Property{}, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{}},
		{[]Property{property, borderProperty}, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{}},
		{[]Property{property, shorterProperty, borderProperty, longProperty, longProperty2}, []string{"Repo", "OtherRepo"}, []FileWithLongProperty{
			{file1, longProperty.valueLength(), longProperty},
			{file2, longProperty.valueLength(), longProperty},
			{file1, longProperty2.valueLength(), longProperty2},
		}},
		{[]Property{property, shorterProperty, borderProperty, longProperty, longProperty2}, []string{"Repo"}, []FileWithLongProperty{
			{file1, longProperty.valueLength(), longProperty},
			{file1, longProperty2.valueLength(), longProperty2},
		}},
	}

	for _, testCase := range cases {
		testSearchPropertiesInFiles(t, testCase.properties, testCase.specificRepos, propertyToFiles, testCase.expected)
	}
}

func testSearchPropertiesInFiles(t *testing.T, properties []Property, specificRepos []string, propertiesFiles map[Property][]api.FileRepresentation, expected []FileWithLongProperty) {
	testServer, serverDetails, _ := getLongPropertyCheckStubServer(t, properties, propertiesFiles, false)
	defer testServer.Close()

	longPropertyCheck := NewLongPropertyCheck(specificRepos, true)
	longPropertyCheck.producerConsumer = parallel.NewRunner(threadCount, maxThreadCapacity, false)
	longPropertyCheck.filesChan = make(chan FileWithLongProperty, threadCount)
	longPropertyCheck.errorsQueue = clientutils.NewErrorsQueue(1)

	var files []FileWithLongProperty
	var waitCollection sync.WaitGroup

	waitCollection.Add(1)
	go func() {
		for current := range longPropertyCheck.filesChan {
			files = append(files, current)
		}
		waitCollection.Done()
	}()

	longPropertyCheck.longPropertiesTaskProducer(nil, precheckrunner.RunArguments{Context: nil, ServerDetails: serverDetails})
	longPropertyCheck.producerConsumer.Done()
	longPropertyCheck.producerConsumer.Run()
	close(longPropertyCheck.filesChan)
	waitCollection.Wait()
	assert.ElementsMatch(t, expected, files)
}

func TestLongPropertyExecuteCheck(t *testing.T) {
	cases := []struct {
		serverProperties       []Property
		specificRepos          []string
		disabledDistinctiveAql bool
		shouldPass             bool
	}{
		{[]Property{}, []string{"Repo", "OtherRepo"}, true, true},
		{[]Property{property, shorterProperty, borderProperty}, []string{"Repo", "OtherRepo"}, false, true},
		{[]Property{property, shorterProperty, borderProperty, longProperty, longProperty2}, []string{"Repo", "OtherRepo"}, true, false},
		{[]Property{property, shorterProperty, borderProperty, longProperty2}, []string{"Repo", "OtherRepo"}, false, false},
		{[]Property{property, shorterProperty, borderProperty, longProperty2}, []string{"OtherRepo"}, true, true},
	}

	for _, testCase := range cases {
		testLongPropertyCheckWithStubServer(t, testCase.serverProperties, testCase.specificRepos, propertyToFiles, testCase.disabledDistinctiveAql, testCase.shouldPass)
	}
}

func testLongPropertyCheckWithStubServer(t *testing.T, serverProperties []Property, specificRepos []string, propertiesFiles map[Property][]api.FileRepresentation, disabledDistinctiveAql, shouldPass bool) {
	testServer, serverDetails, _ := getLongPropertyCheckStubServer(t, serverProperties, propertiesFiles, disabledDistinctiveAql)
	defer testServer.Close()

	longPropertyCheck := NewLongPropertyCheck(specificRepos, disabledDistinctiveAql)
	passed, err := longPropertyCheck.ExecuteCheck(precheckrunner.RunArguments{Context: nil, ServerDetails: serverDetails})
	assert.NoError(t, err)
	assert.Equal(t, shouldPass, passed)
}

func getLongPropertyCheckStubServer(t *testing.T, serverProperties []Property, propertiesFiles map[Property][]api.FileRepresentation, disabledDistinctiveAql bool) (*httptest.Server, *config.ServerDetails, artifactory.ArtifactoryServicesManager) {
	return commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+"api/search/aql" {
			content, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			strContent := string(content)
			if strings.Contains(strContent, "properties.find") {
				// Return all known properties
				result := &AqlPropertySearchResult{serverProperties}
				content, err = json.Marshal(result)
				assert.NoError(t, err)
				_, err = w.Write(content)
				assert.NoError(t, err)
			} else if strings.Contains(strContent, "items.find") {
				// Return all known files with the requested properties
				res := &servicesUtils.AqlSearchResult{}
				for prop, files := range propertiesFiles {
					if strings.Contains(strContent, prop.Key) && strings.Contains(strContent, prop.Value) {
						for _, file := range files {
							res.Results = append(res.Results, servicesUtils.ResultItem{
								Repo: file.Repo,
								Path: file.Path,
								Name: file.Name,
							})
						}
					}
				}
				content, err = json.Marshal(res)
				assert.NoError(t, err)
				_, err = w.Write(content)
				assert.NoError(t, err)
				if disabledDistinctiveAql {
					assert.Contains(t, strContent, ".distinct(false)")
				} else {
					assert.NotContains(t, strContent, ".distinct(false)")
				}
			}
		}
	})
}
