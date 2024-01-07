package utils

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type aqlResult struct {
	Results []*servicesUtils.ResultItem `json:"results,omitempty"`
}

func GetDependenciesFromLatestBuild(servicesManager artifactory.ArtifactoryServicesManager, buildName string) (map[string]*entities.Dependency, error) {
	buildDependencies := make(map[string]*entities.Dependency)
	previousBuild, found, err := servicesManager.GetBuildInfo(services.BuildInfoParams{BuildName: buildName, BuildNumber: servicesUtils.LatestBuildNumberKey})
	if err != nil || !found {
		return buildDependencies, err
	}
	for _, module := range previousBuild.BuildInfo.Modules {
		for _, dependency := range module.Dependencies {
			buildDependencies[dependency.Id] = &entities.Dependency{Id: dependency.Id, Type: dependency.Type,
				Checksum: entities.Checksum{Md5: dependency.Md5, Sha1: dependency.Sha1}}
		}
	}
	return buildDependencies, nil
}

// Get dependency's checksum and type.
func getDependencyInfo(name, ver string, previousBuildDependencies map[string]*entities.Dependency,
	servicesManager artifactory.ArtifactoryServicesManager) (checksum entities.Checksum, fileType string, err error) {
	id := name + ":" + ver
	if dep, ok := previousBuildDependencies[id]; ok {
		// Get checksum from previous build.
		checksum = dep.Checksum
		fileType = dep.Type
		return
	}

	// Get info from Artifactory.
	log.Debug("Fetching checksums for", id)
	var stream io.ReadCloser
	stream, err = servicesManager.Aql(servicesUtils.CreateAqlQueryForYarn(name, ver))
	if err != nil {
		return
	}
	defer func() {
		e := stream.Close()
		if err == nil {
			err = e
		}
	}()
	var result []byte
	result, err = io.ReadAll(stream)
	if err != nil {
		return
	}
	parsedResult := new(aqlResult)
	if err = json.Unmarshal(result, parsedResult); err != nil {
		return entities.Checksum{}, "", errorutils.CheckError(err)
	}
	if len(parsedResult.Results) == 0 {
		log.Debug(id, "could not be found in Artifactory.")
		return
	}
	if i := strings.LastIndex(parsedResult.Results[0].Name, "."); i != -1 {
		fileType = parsedResult.Results[0].Name[i+1:]
	}
	log.Debug(id, "was found in Artifactory. Name:", parsedResult.Results[0].Name,
		"SHA-1:", parsedResult.Results[0].Actual_Sha1,
		"MD5:", parsedResult.Results[0].Actual_Md5)

	checksum = entities.Checksum{Sha1: parsedResult.Results[0].Actual_Sha1, Md5: parsedResult.Results[0].Actual_Md5, Sha256: parsedResult.Results[0].Sha256}
	return
}

func ExtractYarnOptionsFromArgs(args []string) (threads int, detailedSummary, xrayScan bool, scanOutputFormat format.OutputFormat, cleanArgs []string, buildConfig *build.BuildConfiguration, err error) {
	threads = 3
	// Extract threads information from the args.
	flagIndex, valueIndex, numOfThreads, err := coreutils.FindFlag("--threads", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if numOfThreads != "" {
		threads, err = strconv.Atoi(numOfThreads)
		if err != nil {
			err = errorutils.CheckError(err)
			return
		}
	}
	detailedSummary, xrayScan, scanOutputFormat, cleanArgs, buildConfig, err = ExtractNpmOptionsFromArgs(args)
	return
}

func PrintMissingDependencies(missingDependencies []string) {
	if len(missingDependencies) == 0 {
		return
	}

	log.Warn(strings.Join(missingDependencies, "\n"), "\nThe npm dependencies above could not be found in Artifactory and therefore are not included in the build-info.\n"+
		"Deleting the local cache will force populating Artifactory with these dependencies.")
}

func CreateCollectChecksumsFunc(previousBuildDependencies map[string]*entities.Dependency, servicesManager artifactory.ArtifactoryServicesManager, missingDepsChan chan string) func(dependency *entities.Dependency) (bool, error) {
	return func(dependency *entities.Dependency) (bool, error) {
		splitDepId := strings.SplitN(dependency.Id, ":", 2)
		name := splitDepId[0]
		ver := splitDepId[1]

		// Get dependency info.
		checksum, fileType, err := getDependencyInfo(name, ver, previousBuildDependencies, servicesManager)
		if err != nil || checksum.IsEmpty() {
			missingDepsChan <- dependency.Id
			return false, err
		}

		// Update dependency.
		dependency.Type = fileType
		dependency.Checksum = checksum
		return true, nil
	}
}
