package buildinfo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	servicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Artifactory has a max number of character for a single request,
// therefore we limit the maximum number of sha1 and repositories for a single AQL request.
const (
	repoBatchSize = 5
)

var sha1BatchSize = 125

type BuildPublishCommand struct {
	buildConfiguration *utils.BuildConfiguration
	rtDetails          *config.ArtifactoryDetails
	config             *buildinfo.Configuration
	threads            int
}

func NewBuildPublishCommand() *BuildPublishCommand {
	return &BuildPublishCommand{}
}

func (bpc *BuildPublishCommand) SetConfig(config *buildinfo.Configuration) *BuildPublishCommand {
	bpc.config = config
	return bpc
}

func (bpc *BuildPublishCommand) SetThreads(threads int) *BuildPublishCommand {
	bpc.threads = threads
	return bpc
}

func (bpc *BuildPublishCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *BuildPublishCommand {
	bpc.rtDetails = rtDetails
	return bpc
}

func (bpc *BuildPublishCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildPublishCommand {
	bpc.buildConfiguration = buildConfiguration
	return bpc
}

func (bpc *BuildPublishCommand) CommandName() string {
	return "rt_build_publish"
}

func (bpc *BuildPublishCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return bpc.rtDetails, nil
}

func (bpc *BuildPublishCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(bpc.rtDetails, bpc.config.DryRun)
	if err != nil {
		return err
	}

	buildInfo, err := bpc.createBuildInfoFromPartials()
	if err != nil {
		return err
	}

	generatedBuildsInfo, err := utils.GetGeneratedBuildsInfo(bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber)
	if err != nil {
		return err
	}

	if err := bpc.addBuildToDependencies(generatedBuildsInfo); err != nil {
		return err
	}

	for _, v := range generatedBuildsInfo {
		buildInfo.Append(v)
	}

	if err = servicesManager.PublishBuildInfo(buildInfo); err != nil {
		return err
	}

	if !bpc.config.DryRun {
		return utils.RemoveBuildDir(bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber)
	}
	return nil
}

func (bpc *BuildPublishCommand) createBuildInfoFromPartials() (*buildinfo.BuildInfo, error) {
	buildName := bpc.buildConfiguration.BuildName
	buildNumber := bpc.buildConfiguration.BuildNumber
	partials, err := utils.ReadPartialBuildInfoFiles(buildName, buildNumber)
	if err != nil {
		return nil, err
	}
	sort.Sort(partials)

	buildInfo := buildinfo.New()
	buildInfo.SetAgentName(coreutils.GetClientAgent())
	buildInfo.SetAgentVersion(coreutils.GetVersion())
	buildInfo.SetBuildAgentVersion(coreutils.GetVersion())
	buildInfo.SetArtifactoryPluginVersion(coreutils.GetUserAgent())
	buildInfo.Name = buildName
	buildInfo.Number = buildNumber
	buildGeneralDetails, err := utils.ReadBuildInfoGeneralDetails(buildName, buildNumber)
	if err != nil {
		return nil, err
	}
	buildInfo.Started = buildGeneralDetails.Timestamp.Format("2006-01-02T15:04:05.000-0700")
	modules, env, vcs, issues, err := extractBuildInfoData(partials, bpc.config.IncludeFilter(), bpc.config.ExcludeFilter())
	if err != nil {
		return nil, err
	}
	if len(env) != 0 {
		buildInfo.Properties = env
	}
	buildInfo.ArtifactoryPrincipal = bpc.rtDetails.User
	buildInfo.BuildUrl = bpc.config.BuildUrl
	if vcs != (buildinfo.Vcs{}) {
		buildInfo.Revision = vcs.Revision
		buildInfo.Url = vcs.Url
	}
	// Check for Tracker as it must be set
	if issues.Tracker != nil && issues.Tracker.Name != "" {
		buildInfo.Issues = &issues
	}
	for _, module := range modules {
		if module.Id == "" {
			module.Id = buildName
		}
		buildInfo.Modules = append(buildInfo.Modules, module)
	}
	return buildInfo, nil
}

func extractBuildInfoData(partials buildinfo.Partials, includeFilter, excludeFilter buildinfo.Filter) ([]buildinfo.Module, buildinfo.Env, buildinfo.Vcs, buildinfo.Issues, error) {
	var vcs buildinfo.Vcs
	var issues buildinfo.Issues
	env := make(map[string]string)
	partialModules := make(map[string]partialModule)
	issuesMap := make(map[string]*buildinfo.AffectedIssue)
	for _, partial := range partials {
		switch {
		case partial.Artifacts != nil:
			for _, artifact := range partial.Artifacts {
				addArtifactToPartialModule(artifact, partial.ModuleId, partialModules)
			}
		case partial.Dependencies != nil:
			for _, dependency := range partial.Dependencies {
				addDependencyToPartialModule(dependency, partial.ModuleId, partialModules)
			}
		case partial.Vcs != nil:
			vcs = *partial.Vcs
			if partial.Issues == nil {
				continue
			}
			// Collect issues.
			issues.Tracker = partial.Issues.Tracker
			issues.AggregateBuildIssues = partial.Issues.AggregateBuildIssues
			issues.AggregationBuildStatus = partial.Issues.AggregationBuildStatus
			// If affected issues exist, add them to issues map
			if partial.Issues.AffectedIssues != nil {
				for i, issue := range partial.Issues.AffectedIssues {
					issuesMap[issue.Key] = &partial.Issues.AffectedIssues[i]
				}
			}
		case partial.Env != nil:
			envAfterIncludeFilter, e := includeFilter(partial.Env)
			if errorutils.CheckError(e) != nil {
				return partialModulesToModules(partialModules), env, vcs, issues, e
			}
			envAfterExcludeFilter, e := excludeFilter(envAfterIncludeFilter)
			if errorutils.CheckError(e) != nil {
				return partialModulesToModules(partialModules), env, vcs, issues, e
			}
			for k, v := range envAfterExcludeFilter {
				env[k] = v
			}
		}
	}
	return partialModulesToModules(partialModules), env, vcs, issuesMapToArray(issues, issuesMap), nil
}

func partialModulesToModules(partialModules map[string]partialModule) []buildinfo.Module {
	var modules []buildinfo.Module
	for moduleId, singlePartialModule := range partialModules {
		moduleArtifacts := artifactsMapToList(singlePartialModule.artifacts)
		moduleDependencies := dependenciesMapToList(singlePartialModule.dependencies)
		modules = append(modules, *createModule(moduleId, moduleArtifacts, moduleDependencies))
	}
	return modules
}

func issuesMapToArray(issues buildinfo.Issues, issuesMap map[string]*buildinfo.AffectedIssue) buildinfo.Issues {
	for _, issue := range issuesMap {
		issues.AffectedIssues = append(issues.AffectedIssues, *issue)
	}
	return issues
}

func addDependencyToPartialModule(dependency buildinfo.Dependency, moduleId string, partialModules map[string]partialModule) {
	// init map if needed
	if partialModules[moduleId].dependencies == nil {
		partialModules[moduleId] =
			partialModule{artifacts: partialModules[moduleId].artifacts,
				dependencies: make(map[string]buildinfo.Dependency)}
	}
	key := fmt.Sprintf("%s-%s-%s-%s", dependency.Id, dependency.Sha1, dependency.Md5, dependency.Scopes)
	partialModules[moduleId].dependencies[key] = dependency
}

func addArtifactToPartialModule(artifact buildinfo.Artifact, moduleId string, partialModules map[string]partialModule) {
	// init map if needed
	if partialModules[moduleId].artifacts == nil {
		partialModules[moduleId] =
			partialModule{artifacts: make(map[string]buildinfo.Artifact),
				dependencies: partialModules[moduleId].dependencies}
	}
	key := fmt.Sprintf("%s-%s-%s", artifact.Name, artifact.Sha1, artifact.Md5)
	partialModules[moduleId].artifacts[key] = artifact
}

func artifactsMapToList(artifactsMap map[string]buildinfo.Artifact) []buildinfo.Artifact {
	var artifacts []buildinfo.Artifact
	for _, artifact := range artifactsMap {
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func dependenciesMapToList(dependenciesMap map[string]buildinfo.Dependency) []buildinfo.Dependency {
	var dependencies []buildinfo.Dependency
	for _, dependency := range dependenciesMap {
		dependencies = append(dependencies, dependency)
	}
	return dependencies
}

func createModule(moduleId string, artifacts []buildinfo.Artifact, dependencies []buildinfo.Dependency) *buildinfo.Module {
	module := createDefaultModule(moduleId)
	if artifacts != nil && len(artifacts) > 0 {
		module.Artifacts = append(module.Artifacts, artifacts...)
	}
	if dependencies != nil && len(dependencies) > 0 {
		module.Dependencies = append(module.Dependencies, dependencies...)
	}
	return module
}

func createDefaultModule(moduleId string) *buildinfo.Module {
	return &buildinfo.Module{
		Id:           moduleId,
		Properties:   map[string][]string{},
		Artifacts:    []buildinfo.Artifact{},
		Dependencies: []buildinfo.Dependency{},
	}
}

type partialModule struct {
	artifacts    map[string]buildinfo.Artifact
	dependencies map[string]buildinfo.Dependency
}

// The dependencies included in the build-info can be artifacts of other builds (included as artifacts in a previous build-info).
// For dependencies which are the artifacts of other builds, this function finds those builds, so that they can be added to the build-info dependencies.
func (bpc *BuildPublishCommand) addBuildToDependencies(partialsBuildInfo []*buildinfo.PartialBuildInfo) error {
	log.Info("Collecting dependencies' build information... This may take a few minutes...")
	sm, err := utils.CreateServiceManager(bpc.rtDetails, bpc.config.DryRun)
	if err != nil {
		return err
	}
	repositoriesDetails, err := getRepositoriesDetails(sm)
	if err != nil {
		return err
	}
	// Key: dependency sha1, Value: a list of pointers to dependency build property.
	depBuildMap := make(map[string][]*string)
	var searchResults []servicesutils.ResultItem
	for _, partialBuildInfo := range partialsBuildInfo {
		localRepositories, err := filterNonLocalRepos(partialBuildInfo.ResolverRepositories, repositoriesDetails, sm)
		if err != nil {
			return err
		}
		if len(localRepositories) == 0 {
			continue
		}
		// List of dependencies sha1.
		sha1Set := coreutils.NewStringSet()
		for _, module := range partialBuildInfo.Modules {
			for i := 0; i < len(module.Dependencies); i++ {
				dependency := module.Dependencies[i]
				if dependency.Checksum != nil {
					depBuilds := depBuildMap[dependency.Sha1]
					// Sha1 may be present in more than a single module.
					depBuilds = append(depBuilds, &dependency.Build)
					sha1Set.Add(dependency.Sha1)
				}
			}
		}
		if sha1Set.TotalStrings() == 0 {
			continue
		}
		partialSearchResults, err := bpc.getArtifactsPropsBySha1(localRepositories, sha1Set, sm)
		if err != nil {
			return err
		}
		searchResults = append(searchResults, partialSearchResults...)
	}
	// Update the dependencies build.
	for _, searchResult := range searchResults {
		var buildName, buildNumber, timestamp string
		for _, prop := range searchResult.Properties {
			switch prop.Key {
			case "build.name":
				buildName = prop.Value + "/"
			case "build.number":
				buildNumber = prop.Value + "/"
			case "build.timestamp":
				timestamp = prop.Value
			}
		}
		for _, buildPrt := range depBuildMap[searchResult.Actual_Sha1] {
			*buildPrt = buildName + buildNumber + timestamp
		}
	}
	return nil
}

// Search for artifacts properties by sha1 among all the repositories.
// AQL requests have a size limit, therefore, we split the requests into small groups.
func (bpc *BuildPublishCommand) getArtifactsPropsBySha1(repositories []string, sha1s *coreutils.StringSet, sm artifactory.ArtifactoryServicesManager) (totalResults []servicesutils.ResultItem, err error) {
	reposBatches := groupItems(repositories, repoBatchSize)
	for _, repoBach := range reposBatches {
		if sha1s.IsEmpty() {
			break
		}
		sha1Batches := groupItems(sha1s.ToSlice(), sha1BatchSize)
		searchResults := make([]*servicesutils.AqlSearchResult, len(sha1Batches))
		producerConsumer := parallel.NewBounedRunner(bpc.threads, false)
		errorsQueue := clientutils.NewErrorsQueue(1)
		handlerFunc := bpc.createGetArtifactsPropsBySha1Func(repoBach, sm, searchResults)
		go func() {
			defer producerConsumer.Done()
			for i, sha1Bach := range sha1Batches {
				producerConsumer.AddTaskWithError(handlerFunc(sha1Bach, i), errorsQueue.AddError)
			}
		}()
		producerConsumer.Run()
		if err := errorsQueue.GetError(); err != nil {
			return nil, err
		}
		for _, batchResult := range searchResults {
			if batchResult == nil || batchResult.Results == nil {
				continue
			}
			totalResults = append(totalResults, batchResult.Results...)
			// Delete the sha1 that have already been found.
			for _, result := range batchResult.Results {
				sha1s.Delete(result.Actual_Sha1)
			}
		}
	}
	return totalResults, nil
}

// Creates a function that fetches dependency data from Artifactory.
func (bpc *BuildPublishCommand) createGetArtifactsPropsBySha1Func(repoBach []string, sm artifactory.ArtifactoryServicesManager, searchResult []*servicesutils.AqlSearchResult) func(sha1s []string, index int) parallel.TaskFunc {
	return func(sha1s []string, index int) parallel.TaskFunc {
		return func(threadId int) error {
			start := time.Now()
			stream, err := sm.Aql(servicesutils.CreateSearchBySha1AndRepoAqlQuery(repoBach, sha1s))
			t := time.Now()
			elapsed := t.Sub(start)
			if err != nil {
				return err
			}
			defer stream.Close()
			log.Debug(clientutils.GetLogMsgPrefix(threadId, false), "Finished searching artifacts properties by sha1 in", repoBach, ". Took ", elapsed.Seconds(), " seconds to complete the operation.\n")
			result, err := ioutil.ReadAll(stream)
			if err != nil {
				return errorutils.CheckError(err)
			}
			parsedResult := new(servicesutils.AqlSearchResult)
			err = json.Unmarshal(result, &parsedResult)
			if err = errorutils.CheckError(err); err != nil {
				return err
			}
			if len(parsedResult.Results) > 0 {
				searchResult[index] = parsedResult
			}
			return nil
		}
	}
}

// Group large slice into small groups, for example :
// sliceToGroup = []string{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}; groupSize = 3
// returns : [['0' '1' '2'] ['3' '4' '5'] ['6' '7' '8'] ['9]]
func groupItems(sliceToGroup []string, groupSize int) [][]string {
	var groups [][]string
	if groupSize > len(sliceToGroup) {
		return append(groups, sliceToGroup)
	}
	for groupSize < len(sliceToGroup) {
		sliceToGroup, groups = sliceToGroup[groupSize:], append(groups, sliceToGroup[0:groupSize:groupSize])
	}
	return append(groups, sliceToGroup)
}

// Returns only local repositories from 'repositories' including local repositories inside virtual repositories.
func filterNonLocalRepos(repositories []string, repositoriesDetails map[string]*services.RepositoryDetails, sm artifactory.ArtifactoryServicesManager) ([]string, error) {
	if repositories == nil || len(repositories) == 0 {
		return nil, nil
	}
	filteredRepositories := coreutils.NewStringSet()
	for _, repo := range repositories {
		if repositoriesDetails[repo] == nil {
			continue
		}
		switch strings.ToLower(repositoriesDetails[repo].Rclass) {
		case "local":
			filteredRepositories.Add(repo)
		case "virtual":
			virtualRepoDetails, err := sm.GetRepository(repo)
			if err != nil {
				return nil, err
			}
			if virtualRepoDetails != nil && len(virtualRepoDetails.Repositories) > 0 {
				localRepos, err := filterNonLocalRepos(virtualRepoDetails.Repositories, repositoriesDetails, sm)
				if err != nil {
					return nil, err
				}
				filteredRepositories.AddAll(localRepos...)
			}
		}
	}
	return filteredRepositories.ToSlice(), nil
}

// Get a list of all accessable repositories in Artifactory.
// Return a map of:
// Key: repo key, Value: repo details.
func getRepositoriesDetails(sm artifactory.ArtifactoryServicesManager) (map[string]*services.RepositoryDetails, error) {
	var repositoriesDetails map[string]*services.RepositoryDetails
	repositoriesSearchResults, err := sm.GetRepositories()
	if err != nil {
		return nil, err
	}
	repositoriesDetails = make(map[string]*services.RepositoryDetails)
	for _, repo := range repositoriesSearchResults {
		repositoriesDetails[repo.Key] = repo
	}
	return repositoriesDetails, nil
}
