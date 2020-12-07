package dependencies

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"strings"
)

const aqlFilePart = `{"$and":[{` +
	`"path":{"$match":"*"},` +
	`"name":{"$match":"%s"}` +
	`}]},`
const aqlBulkSize = 50

// Create project's 'buildinfo.Dependency' structs for all dependencies.
// 'dependencyToFileMap' contains a mapping between each dependency (package-name) to its actual file (tar.gz, zip, whl etc).
// If a dependency was downloaded in this pip-install execution, checksum will be fetched from Artifactory.
// Otherwise, check if the information exists in the cache.
// Return each package and its dependency struct, and a list of all the dependencies which their information could not be obtained.
func GetDependencies(dependencyToFileMap map[string]string, dependenciesCache *DependenciesCache, servicesManager artifactory.ArtifactoryServicesManager, repository string) (dependenciesMap map[string]*buildinfo.Dependency, missingDeps []string, err error) {
	dependenciesMap = make(map[string]*buildinfo.Dependency)
	getFromArtifactoryMap := make(map[string]string)

	for depName, depFileName := range dependencyToFileMap {
		if depFileName != "" {
			// Dependency downloaded from Artifactory.
			getFromArtifactoryMap[depFileName] = depName
			continue
		}
		// Dependency wasn't downloaded in this execution, check cache for dependency info.
		if dependenciesCache != nil {
			dep := dependenciesCache.GetDependency(depName)
			if dep != nil {
				// Dependency found in cache.
				dependenciesMap[depName] = dep
				continue
			}
		}
		// Dependency wasn't downloaded from Artifactory nor found in cache.
		missingDeps = append(missingDeps, depName)
	}

	missingDependenciesFromArtifactory, err := addDependenciesFromArtifactory(dependenciesMap, getFromArtifactoryMap, repository, servicesManager)
	if err != nil {
		return nil, nil, err
	}
	missingDeps = append(missingDeps, missingDependenciesFromArtifactory...)

	return dependenciesMap, missingDeps, nil
}

// Get the checksums for the files in 'fileToPackageMap' from Artifactory.
// Create a dependency struct for each file, and update 'dependenciesMap'.
// Return the packages which couldn't be found in Artifactory.
func addDependenciesFromArtifactory(dependenciesMap map[string]*buildinfo.Dependency, fileToPackageMap map[string]string,
	repository string, servicesManager artifactory.ArtifactoryServicesManager) (missingDeps []string, err error) {
	if len(fileToPackageMap) < 1 {
		return
	}

	aqlQueries := createAqlQueries(fileToPackageMap, repository, aqlBulkSize)
	queriesResults, err := getQueriesResults(aqlQueries, servicesManager)
	if err != nil {
		return
	}
	for _, result := range queriesResults {
		dependency := createDependencyFromResult(result, fileToPackageMap)
		if dependency != nil {
			dependenciesMap[fileToPackageMap[result.Name]] = dependency
		}
	}

	// Collect missing dependencies.
	for fileName, packageName := range fileToPackageMap {
		if _, ok := dependenciesMap[packageName]; !ok {
			log.Debug(fmt.Sprintf("Failed getting checksums from Artifactory for file: '%s'", fileName))
			missingDeps = append(missingDeps, packageName)
		}
	}

	return
}

// Create a Dependency from the info received in 'result'.
// A result from Artifactory contains the file-name and checksums.
func createDependencyFromResult(result *results, fileToPackageMap map[string]string) *buildinfo.Dependency {
	fileName := result.Name
	if _, ok := fileToPackageMap[fileName]; !ok {
		return nil
	}
	sha1 := result.Actual_sha1
	md5 := result.Actual_md5
	if sha1 == "" || md5 == "" {
		return nil
	}
	fileType := ""
	if i := strings.LastIndex(fileName, "."); i != -1 {
		fileType = fileName[i+1:]
	}
	log.Debug(fmt.Sprintf("Found checksums for file: %s, sha1: '%s', md5: '%s'", fileName, sha1, md5))
	return &buildinfo.Dependency{Id: fileName, Checksum: &buildinfo.Checksum{Sha1: sha1, Md5: md5}, Type: fileType}
}

func createAqlQueries(fileToPackageMap map[string]string, repository string, bulkSize int) []string {
	var aqlQueries []string
	var querySb strings.Builder
	filesCounter := 0
	for fileName := range fileToPackageMap {
		filesCounter++
		querySb.WriteString(fmt.Sprintf(aqlFilePart, fileName))
		if filesCounter == bulkSize {
			aqlQueries = append(aqlQueries, serviceutils.CreateAqlQueryForPypi(repository, strings.TrimSuffix(querySb.String(), ",")))
			filesCounter = 0
			querySb.Reset()
		}
	}
	if querySb.Len() > 0 {
		aqlQueries = append(aqlQueries, serviceutils.CreateAqlQueryForPypi(repository, strings.TrimSuffix(querySb.String(), ",")))
	}
	return aqlQueries
}

func getQueriesResults(aqlQueries []string, servicesManager artifactory.ArtifactoryServicesManager) ([]*results, error) {
	var results []*results
	for _, query := range aqlQueries {
		// Run query.
		queryResult, err := runQuery(query, servicesManager)
		if err != nil {
			return nil, err
		}
		// Append results.
		if len(queryResult.Results) > 0 {
			results = append(results, queryResult.Results...)
		}
	}
	return results, nil
}

func runQuery(query string, servicesManager artifactory.ArtifactoryServicesManager) (*aqlResult, error) {
	// Run query.
	stream, err := servicesManager.Aql(query)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// Read results.
	result, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	parsedResult := new(aqlResult)
	err = json.Unmarshal(result, parsedResult)
	if err = errorutils.CheckError(err); err != nil {
		return nil, err
	}
	return parsedResult, nil
}

type aqlResult struct {
	Results []*results `json:"results,omitempty"`
}

type results struct {
	Actual_sha1 string `json:"actual_sha1,omitempty"`
	Actual_md5  string `json:"actual_md5,omitempty"`
	Name        string `json:"name,omitempty"`
}
