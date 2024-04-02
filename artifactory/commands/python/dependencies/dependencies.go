package dependencies

import (
	"encoding/json"
	"fmt"
	ioutils "github.com/jfrog/gofrog/io"
	"io"
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
func UpdateDepsChecksumInfo(dependenciesMap map[string]buildinfo.Dependency, srcPath string, servicesManager artifactory.ArtifactoryServicesManager, repository string) error {
	dependenciesCache, err := GetProjectDependenciesCache(srcPath)
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
		depInfo.Checksum = depChecksum
		dependenciesMap[depName] = depInfo
	}

	promptMissingDependencies(missingDeps)

	err = UpdateDependenciesCache(dependenciesMap, srcPath)
	if err != nil {
		return err
	}
	return nil
}

// Get dependency information.
// If dependency was downloaded in this pip-install execution, fetch info from Artifactory.
// Otherwise, fetch info from cache.
func getDependencyInfo(depName, repository string, dependenciesCache *DependenciesCache, depFileName string, servicesManager artifactory.ArtifactoryServicesManager) (string, buildinfo.Checksum, error) {
	// Check if this dependency was updated during this pip-install execution, and we have its file-name.
	// If updated - fetch checksum from Artifactory, regardless of what was previously stored in cache.

	if dependenciesCache != nil {
		depFromCache := dependenciesCache.GetDependency(depName)
		if depFromCache.Id != "" {
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
	defer ioutils.Close(stream, &err)
	result, err := io.ReadAll(stream)
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
	sha256 := parsedResult.Results[0].Sha256
	sha1 := parsedResult.Results[0].Actual_Sha1
	md5 := parsedResult.Results[0].Actual_Md5
	if sha1 == "" || md5 == "" {
		// Missing checksum.
		log.Debug(fmt.Sprintf("Missing checksums for file: %s, sha256: '%s', sha1: '%s', md5: '%s'", dependencyFile, sha256, sha1, md5))
		return
	}

	// Update checksum.
	checksum = buildinfo.Checksum{Sha256: sha256, Sha1: sha1, Md5: md5}
	log.Debug(fmt.Sprintf("Found checksums for file: %s, sha256: '%s', sha1: '%s', md5: '%s'", dependencyFile, sha256, sha1, md5))

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
	Results []*serviceutils.ResultItem `json:"results,omitempty"`
}
