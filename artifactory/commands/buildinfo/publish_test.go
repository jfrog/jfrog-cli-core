package buildinfo

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
)

var envVars = map[string]string{"KeY": "key_val", "INClUdEd_VaR": "included_var", "EXCLUDED_pASSwoRd_var": "excluded_var"}

func TestIncludeAllPattern(t *testing.T) {
	conf := buildinfo.Configuration{EnvInclude: "*"}
	includeFilter := conf.IncludeFilter()
	filteredKeys, err := includeFilter(envVars)
	if err != nil {
		t.Error(err)
	}

	equals := reflect.DeepEqual(envVars, filteredKeys)
	if !equals {
		t.Error("expected:", envVars, "got:", filteredKeys)
	}
}

func TestIncludePartial(t *testing.T) {
	conf := buildinfo.Configuration{EnvInclude: "*ED_V*;EXC*SwoRd_var"}
	includeFilter := conf.IncludeFilter()
	filteredKeys, err := includeFilter(envVars)
	if err != nil {
		t.Error(err)
	}

	expected := map[string]string{"INClUdEd_VaR": "included_var", "EXCLUDED_pASSwoRd_var": "excluded_var"}
	equals := reflect.DeepEqual(expected, filteredKeys)
	if !equals {
		t.Error("expected:", expected, "got:", filteredKeys)
	}
}

func TestIncludePartialIgnoreCase(t *testing.T) {
	conf := buildinfo.Configuration{EnvInclude: "*Ed_v*"}
	includeFilter := conf.IncludeFilter()
	filteredKeys, err := includeFilter(envVars)
	if err != nil {
		t.Error(err)
	}

	expected := map[string]string{"INClUdEd_VaR": "included_var"}
	equals := reflect.DeepEqual(expected, filteredKeys)
	if !equals {
		t.Error("expected:", expected, "got:", filteredKeys)
	}
}

func TestExcludePasswordsPattern(t *testing.T) {
	conf := buildinfo.Configuration{EnvExclude: "*paSSword*;*PsW*;*seCrEt*;*kEy*;*token*"}
	excludeFilter := conf.ExcludeFilter()
	filteredKeys, err := excludeFilter(envVars)
	if err != nil {
		t.Error(err)
	}

	expected := map[string]string{"INClUdEd_VaR": "included_var"}
	equals := reflect.DeepEqual(expected, filteredKeys)
	if !equals {
		t.Error("expected:", expected, "got:", filteredKeys)
	}
}

func TestGroupItems(t *testing.T) {
	slice := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	results := groupItems(slice, 3)
	assert.ElementsMatch(t, results, [][]string{{"0", "1", "2"}, {"3", "4", "5"}, {"6", "7", "8"}, {"9"}})
}

type publishServiceManagerMock struct {
	artifactory.EmptyArtifactoryServicesManager
}

func (psmm *publishServiceManagerMock) GetRepositories() ([]*services.RepositoryDetails, error) {
	testDataPath, err := getTestDataPath()
	if err != nil {
		return nil, err
	}
	var allRepositoriesDetailsSlice []*services.RepositoryDetails
	err = loadFromFile(filepath.Join(testDataPath, "allrepositoriesdetails.json"), &allRepositoriesDetailsSlice)
	return allRepositoriesDetailsSlice, err
}

func (psmm *publishServiceManagerMock) Aql(aql string) (io.ReadCloser, error) {
	switch {
	case strings.Contains(aql, "1"):
		return ioutil.NopCloser(strings.NewReader("shiny!")), nil
	case strings.Contains(aql, "2"):
		return ioutil.NopCloser(strings.NewReader("shiny!")), nil

	case strings.Contains(aql, "3"):
		return ioutil.NopCloser(strings.NewReader("shiny!")), nil

	}
	return nil, nil
}

func (psmm *publishServiceManagerMock) GetRepository(repoKey string) (*services.RepositoryDetails, error) {
	testDataPath, err := getTestDataPath()
	if err != nil {
		return nil, err
	}
	var virtualDetails *services.RepositoryDetails
	switch repoKey {
	case "virtual-repo":
		err = loadFromFile(filepath.Join(testDataPath, "firstvirtualdetails.json"), &virtualDetails)
	case "another-virtual-repo":
		err = loadFromFile(filepath.Join(testDataPath, "secondvirtualdetails.json"), &virtualDetails)
	}
	return virtualDetails, err
}

func TestFilterNonLocalRepos(t *testing.T) {
	localRepo, remoteRepo, virtualRepo := "local-repo", "remote-repo", "virtual-repo"
	smMock := new(publishServiceManagerMock)
	repositoriesDetails, err := getRepositoriesDetails(smMock)
	assert.NoError(t, err)
	localRepos, err := filterNonLocalRepos([]string{localRepo}, repositoriesDetails, smMock)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"local-repo"}, localRepos)

	localRepos, err = filterNonLocalRepos([]string{remoteRepo}, repositoriesDetails, smMock)
	assert.NoError(t, err)
	assert.Len(t, localRepos, 0)

	localRepos, err = filterNonLocalRepos([]string{localRepo, remoteRepo}, repositoriesDetails, smMock)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"local-repo"}, localRepos)

	localRepos, err = filterNonLocalRepos([]string{virtualRepo}, repositoriesDetails, smMock)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"another-local-repo"}, localRepos)

	localRepos, err = filterNonLocalRepos([]string{localRepo, virtualRepo}, repositoriesDetails, smMock)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"local-repo", "another-local-repo"}, localRepos)
}

func loadFromFile(filePath string, loadInto interface{}) error {
	f, err := os.Open(filepath.Join(filePath))
	if err != nil {
		return err
	}
	defer f.Close()
	byteValue, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(byteValue, loadInto)
}

func getTestDataPath() (string, error) {
	dir, err := os.Getwd()
	return filepath.Join(dir, "..", "testdata", "buildinfo"), err
}

func TestGetRepositoriesDetails(t *testing.T) {
	smMock := new(publishServiceManagerMock)
	repositoriesDetails, err := getRepositoriesDetails(smMock)
	assert.NoError(t, err)
	assert.Len(t, repositoriesDetails, 6)
	for key, repo := range repositoriesDetails {
		assert.Equal(t, key, repo.Key)
	}
}
