package dependencies

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	buildinfo "github.com/jfrog/build-info-go/entities"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Populate project's dependencies with checksums and file names.
// If the dependency was downloaded in this pip-install execution, checksum will be fetched from Artifactory.
// Otherwise, check if exists in cache.
// Return dependency-names of all dependencies which its information could not be obtained.
func UpdateDepsChecksumInfo(dependenciesMap map[string]*buildinfo.Dependency, cacheDirPath string, servicesManager artifactory.ArtifactoryServicesManager, repository string) error {
	dependenciesCache, err := GetProjectDependenciesCache(cacheDirPath)
	if err != nil {
		return err
	}

	var missingDeps []string
	// Iterate dependencies map to update info.
	for depName, depInfo := range dependenciesMap {
		// Get dependency info.
		depFileName, depChecksum, err := getDependencyInfo(depName, repository, dependenciesCache, depInfo.Id, servicesManager)
		if err != nil {
			return err
		}

		// Check if info not found.
		if depFileName == "" || depChecksum.IsEmpty() {
			// Dependency either wasn't downloaded in this run nor stored in cache.
			missingDeps = append(missingDeps, depName)
			// dependenciesMap should contain only dependencies with checksums.
			delete(dependenciesMap, depName)

			continue
		}
		fileType := ""
		// Update dependency info.
		dependenciesMap[depName].Id = depFileName
		if i := strings.LastIndex(depFileName, "."); i != -1 {
			fileType = depFileName[i+1:]
		}
		dependenciesMap[depName].Type = fileType
		dependenciesMap[depName].Checksum = depChecksum
	}

	promptMissingDependencies(missingDeps)
	return nil
}

// Before running this function, dependency IDs may be the file names of the resolved python packages.
// Update build info dependency IDs and the requestedBy field.
// allDependencies      - Dependency name to Dependency map
// dependenciesGraph    - Dependency graph as built by 'pipdeptree' or 'pipenv graph'
// topLevelPackagesList - The direct dependencies
// packageName          - The resolved package name of the Python project, may be empty if we couldn't resolve it
// moduleName           - The input module name from the user, or the packageName
func UpdateDepsIdsAndRequestedBy(allDependencies map[string]*buildinfo.Dependency, dependenciesGraph map[string][]string,
	topLevelPackagesList []string, packageName, moduleName string) {
	if packageName == "" {
		// Projects without setup.py
		dependenciesGraph[moduleName] = topLevelPackagesList
	} else {
		// Projects with setup.py
		dependenciesGraph[moduleName] = dependenciesGraph[packageName]
	}
	rootModule := buildinfo.Dependency{Id: moduleName, RequestedBy: [][]string{{}}}
	updateDepsIdsAndRequestedBy(rootModule, allDependencies, dependenciesGraph)
}

func updateDepsIdsAndRequestedBy(parentDependency buildinfo.Dependency, dependenciesMap map[string]*buildinfo.Dependency, dependenciesGraph map[string][]string) {
	childrenList := dependenciesGraph[parentDependency.Id]
	for _, childName := range childrenList {
		childKey := childName[0:strings.Index(childName, ":")]
		if childDep, ok := dependenciesMap[childKey]; ok {
			for _, parentRequestedBy := range parentDependency.RequestedBy {
				childRequestedBy := append([]string{parentDependency.Id}, parentRequestedBy...)
				childDep.RequestedBy = append(childDep.RequestedBy, childRequestedBy)
			}
			if childDep.NodeHasLoop() {
				continue
			}
			childDep.Id = childName
			// Run recursive call on child dependencies
			updateDepsIdsAndRequestedBy(*childDep, dependenciesMap, dependenciesGraph)
		}
	}
}

// Get dependency information.
// If dependency was downloaded in this pip-install execution, fetch info from Artifactory.
// Otherwise, fetch info from cache.
func getDependencyInfo(depName, repository string, dependenciesCache *DependenciesCache, depFileName string, servicesManager artifactory.ArtifactoryServicesManager) (string, buildinfo.Checksum, error) {
	// Check if this dependency was updated during this pip-install execution, and we have its file-name.
	// If updated - fetch checksum from Artifactory, regardless of what was previously stored in cache.

	if dependenciesCache != nil {
		depFromCache := dependenciesCache.GetDependency(depName)
		if depFromCache != nil {
			// Cached dependencies are used in the following cases:
			// 	1. When file name is empty and therefore the dependency is cached
			// 	2. When file name is identical to the cached file name
			if depFileName == "" || depFileName == depFromCache.Id {
				// The checksum was found in cache - the info is returned.
				return depFromCache.Id, depFromCache.Checksum, nil
			}
		}
	}

	if depFileName != "" {
		checksum, err := getDependencyChecksumFromArtifactory(servicesManager, repository, depFileName)
		return depFileName, checksum, err
	}

	return "", buildinfo.Checksum{}, nil
}

// Fetch checksum for file from Artifactory.
// If the file isn't found, or md5 or sha1 are missing, return nil.
func getDependencyChecksumFromArtifactory(servicesManager artifactory.ArtifactoryServicesManager, repository, dependencyFile string) (checksum buildinfo.Checksum, err error) {
	log.Debug(fmt.Sprintf("Fetching checksums for: %s", dependencyFile))
	repository, err = utils.GetRepoNameForDependenciesSearch(repository, servicesManager)
	if err != nil {
		return
	}
	stream, err := servicesManager.Aql(serviceutils.CreateAqlQueryForPypi(repository, dependencyFile))
	if err != nil {
		return
	}
	defer func() {
		e := stream.Close()
		if err == nil {
			err = e
		}
	}()
	result, err := ioutil.ReadAll(stream)
	if err != nil {
		return
	}
	parsedResult := new(aqlResult)
	err = json.Unmarshal(result, parsedResult)
	if err = errorutils.CheckError(err); err != nil {
		return
	}
	if len(parsedResult.Results) == 0 {
		log.Debug(fmt.Sprintf("File: %s could not be found in repository: %s", dependencyFile, repository))
		return
	}

	// Verify checksum exist.
	sha1 := parsedResult.Results[0].Actual_sha1
	md5 := parsedResult.Results[0].Actual_md5
	if sha1 == "" || md5 == "" {
		// Missing checksum.
		log.Debug(fmt.Sprintf("Missing checksums for file: %s, sha1: '%s', md5: '%s'", dependencyFile, sha1, md5))
		return
	}

	// Update checksum.
	checksum = buildinfo.Checksum{Sha1: sha1, Md5: md5}
	log.Debug(fmt.Sprintf("Found checksums for file: %s, sha1: '%s', md5: '%s'", dependencyFile, sha1, md5))

	return
}

func promptMissingDependencies(missingDeps []string) {
	if len(missingDeps) > 0 {
		log.Warn(strings.Join(missingDeps, "\n"))
		log.Warn("The pypi packages above could not be found in Artifactory or were not downloaded in this execution, therefore they are not included in the build-info.\n" +
			"Reinstalling in clean environment or using '--no-cache-dir' and '--force-reinstall' flags (in one execution only), will force downloading and populating Artifactory with these packages, and therefore resolve the issue.")
	}
}

type aqlResult struct {
	Results []*results `json:"results,omitempty"`
}

type results struct {
	Actual_md5  string `json:"actual_md5,omitempty"`
	Actual_sha1 string `json:"actual_sha1,omitempty"`
}
