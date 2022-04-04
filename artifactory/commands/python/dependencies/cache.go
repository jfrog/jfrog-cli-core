package dependencies

import (
	"encoding/json"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"os"
	"path/filepath"
)

const cacheLatestVersion = 1

type DependenciesCache struct {
	Version int                             `json:"version,omitempty"`
	DepsMap map[string]buildinfo.Dependency `json:"dependencies,omitempty"`
}

// Reads the json cache file of recent used project's dependencies,  and converts it into a map of
// Key: dependency_name Value: dependency's struct with all relevant information.
// If cache file does not exist -> return nil, nil.
// If error occurred, return error.
func GetProjectDependenciesCache(cacheDir string) (cache *DependenciesCache, err error) {
	cache = new(DependenciesCache)
	cacheFilePath, exists, err := getCacheFilePath(cacheDir)
	if errorutils.CheckError(err) != nil || !exists {
		return nil, err
	}
	jsonFile, err := os.Open(cacheFilePath)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	defer func() {
		e := jsonFile.Close()
		if err == nil {
			err = e
		}
	}()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	err = json.Unmarshal(byteValue, cache)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	return
}

// Receives map of all current project's dependencies information.
// The map contains the dependencies retrieved from Artifactory as well as those read from cache.
// Writes the updated project's dependencies cache with all current dependencies.
func UpdateDependenciesCache(updatedMap map[string]buildinfo.Dependency, cacheDir string) (err error) {
	updatedCache := DependenciesCache{Version: cacheLatestVersion, DepsMap: updatedMap}
	content, err := json.Marshal(&updatedCache)
	if err != nil {
		return errorutils.CheckError(err)
	}
	cacheFilePath, _, err := getCacheFilePath(cacheDir)
	if err != nil {
		return errorutils.CheckError(err)
	}

	cacheFile, err := os.Create(cacheFilePath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := cacheFile.Close()
		if err == nil {
			err = e
		}
	}()
	_, err = cacheFile.Write(content)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return
}

// Return required dependency from cache.
// If dependency does not exist, return nil.
// dependencyName - Name of dependency (lowercase package name).
func (cache DependenciesCache) GetDependency(dependencyName string) (dependency buildinfo.Dependency) {
	dependency = cache.DepsMap[dependencyName]
	return
}

// Cache file will be located in the ./.jfrog/projects/deps.cache.json
func getCacheFilePath(cacheDir string) (cacheFilePath string, exists bool, err error) {
	if errorutils.CheckError(err) != nil {
		return "", false, err
	}
	projectsDirPath := filepath.Join(cacheDir, ".jfrog", "projects")
	err = fileutils.CreateDirIfNotExist(projectsDirPath)
	if err != nil {
		return "", false, err
	}
	cacheFilePath = filepath.Join(projectsDirPath, "deps.cache.json")
	exists, err = fileutils.IsFileExists(cacheFilePath, false)

	return
}
