package buildinfo

import (
	"errors"
	regxp "regexp"
	"strconv"

	buildinfo "github.com/jfrog/build-info-go/entities"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAddDependenciesCommand struct {
	buildConfiguration *utils.BuildConfiguration
	dependenciesSpec   *spec.SpecFiles
	dryRun             bool
	result             *commandsutils.Result
	serverDetails      *config.ServerDetails
}

func NewBuildAddDependenciesCommand() *BuildAddDependenciesCommand {
	return &BuildAddDependenciesCommand{result: new(commandsutils.Result)}
}

func (badc *BuildAddDependenciesCommand) Result() *commandsutils.Result {
	return badc.result
}

func (badc *BuildAddDependenciesCommand) CommandName() string {
	return "rt_build_add_dependencies"
}

func (badc *BuildAddDependenciesCommand) ServerDetails() (*config.ServerDetails, error) {
	if badc.serverDetails != nil {
		return badc.serverDetails, nil
	}
	return config.GetDefaultServerConf()
}

func (badc *BuildAddDependenciesCommand) Run() error {
	log.Info("Running Build Add Dependencies command...")
	var success int
	var fail int
	var err error
	if !badc.dryRun {
		buildName, err := badc.buildConfiguration.GetBuildName()
		if err != nil {
			return err
		}
		buildNumber, err := badc.buildConfiguration.GetBuildNumber()
		if err != nil {
			return err
		}
		if err = utils.SaveBuildGeneralDetails(buildName, buildNumber, badc.buildConfiguration.GetProject()); err != nil {
			return err
		}
	}
	if badc.serverDetails != nil {
		log.Debug("Searching dependencies on Artifactory...")
		success, fail, err = badc.collectRemoteDependencies()
	} else {
		log.Debug("Searching dependencies on local file system...")
		success, fail, err = badc.collectLocalDependencies()
	}
	badc.result.SetSuccessCount(success)
	badc.result.SetFailCount(fail)
	return err
}

func (badc *BuildAddDependenciesCommand) SetDryRun(dryRun bool) *BuildAddDependenciesCommand {
	badc.dryRun = dryRun
	return badc
}

func (badc *BuildAddDependenciesCommand) SetDependenciesSpec(dependenciesSpec *spec.SpecFiles) *BuildAddDependenciesCommand {
	badc.dependenciesSpec = dependenciesSpec
	return badc
}

func (badc *BuildAddDependenciesCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildAddDependenciesCommand {
	badc.serverDetails = serverDetails
	return badc
}

func (badc *BuildAddDependenciesCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildAddDependenciesCommand {
	badc.buildConfiguration = buildConfiguration
	return badc
}

func collectDependenciesChecksums(dependenciesPaths map[string]string) (map[string]*fileutils.FileDetails, int) {
	failures := 0
	dependenciesDetails := make(map[string]*fileutils.FileDetails)
	for _, dependencyPath := range dependenciesPaths {
		var details *fileutils.FileDetails
		var err error
		if fileutils.IsPathSymlink(dependencyPath) {
			log.Info("Adding symlink dependency:", dependencyPath)
			details, err = fspatterns.CreateSymlinkFileDetails()
		} else {
			log.Info("Adding dependency:", dependencyPath)
			details, err = fileutils.GetFileDetails(dependencyPath, true)
		}
		if err != nil {
			log.Error(err)
			failures++
			continue
		}
		dependenciesDetails[dependencyPath] = details
	}
	return dependenciesDetails, failures
}

func (badc *BuildAddDependenciesCommand) collectLocalDependencies() (success, fail int, err error) {
	var dependenciesDetails map[string]*fileutils.FileDetails
	dependenciesPaths, errorOccurred := badc.doCollectLocalDependencies()
	dependenciesDetails, fail = collectDependenciesChecksums(dependenciesPaths)
	if !badc.dryRun {
		buildInfoDependencies := convertFileInfoToDependencies(dependenciesDetails)
		err = badc.savePartialBuildInfo(buildInfoDependencies)
		if err != nil {
			// Mark all as failures.
			fail = len(dependenciesDetails)
			return
		}
	}
	success = len(dependenciesDetails)
	if errorOccurred || fail > 0 {
		err = errors.New("build Add Dependencies command finished with errors. Please review the logs")
	}
	return
}

func (badc *BuildAddDependenciesCommand) collectRemoteDependencies() (success, fail int, err error) {
	servicesManager, err := utils.CreateServiceManager(badc.serverDetails, -1, 0, false)
	if err != nil {
		return
	}
	reader, err := searchItems(badc.dependenciesSpec, servicesManager)
	if err != nil {
		return
	}
	success, fail, err = badc.readRemoteDependencies(reader)
	return
}

func (badc *BuildAddDependenciesCommand) doCollectLocalDependencies() (map[string]string, bool) {
	errorOccurred := false
	dependenciesPaths := make(map[string]string)
	for _, specFile := range badc.dependenciesSpec.Files {
		params, err := prepareArtifactoryParams(specFile)
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		paths, err := getLocalDependencies(params)
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		for _, path := range paths {
			log.Debug("Found matching path:", path)
			dependenciesPaths[path] = path
		}
	}
	return dependenciesPaths, errorOccurred
}

func (badc *BuildAddDependenciesCommand) readRemoteDependencies(reader *content.ContentReader) (success, fail int, err error) {
	if badc.dryRun {
		success, err = reader.Length()
		return
	}
	count := 0
	var buildInfoDependencies []buildinfo.Dependency
	for resultItem := new(specutils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(specutils.ResultItem) {
		buildInfoDependencies = append(buildInfoDependencies, resultItem.ToDependency())
		count++
		if count > clientutils.MaxBufferSize {
			if err = badc.savePartialBuildInfo(buildInfoDependencies); err != nil {
				return
			}
			success += count
			count = 0
			buildInfoDependencies = nil
		}
	}
	if err = reader.GetError(); err != nil {
		return
	}
	if count > 0 {
		if err = badc.savePartialBuildInfo(buildInfoDependencies); err != nil {
			fail += len(buildInfoDependencies)
			return
		}
	}
	success += count
	return
}

func prepareArtifactoryParams(specFile spec.File) (*specutils.CommonParams, error) {
	params, err := specFile.ToCommonParams()
	if err != nil {
		return nil, err
	}

	recursive, err := clientutils.StringToBool(specFile.Recursive, true)
	if err != nil {
		return nil, err
	}

	params.Recursive = recursive
	regexp, err := clientutils.StringToBool(specFile.Regexp, false)
	if err != nil {
		return nil, err
	}

	params.Regexp = regexp
	return params, nil
}

func getLocalDependencies(addDepsParams *specutils.CommonParams) ([]string, error) {
	addDepsParams.SetPattern(clientutils.ReplaceTildeWithUserHome(addDepsParams.GetPattern()))
	// Save parentheses index in pattern, witch have corresponding placeholder.
	rootPath, err := fspatterns.GetRootPath(addDepsParams.GetPattern(), addDepsParams.GetTarget(), "", addDepsParams.GetPatternType(), false)
	if err != nil {
		return nil, err
	}

	isDir, err := fileutils.IsDirExists(rootPath, false)
	if err != nil {
		return nil, err
	}

	if !isDir || fileutils.IsPathSymlink(addDepsParams.GetPattern()) {
		artifact, err := fspatterns.GetSingleFileToUpload(rootPath, "", false)
		if err != nil {
			return nil, err
		}
		return []string{artifact.LocalPath}, nil
	}
	return collectPatternMatchingFiles(addDepsParams, rootPath)
}

func collectPatternMatchingFiles(addDepsParams *specutils.CommonParams, rootPath string) ([]string, error) {
	addDepsParams.SetPattern(clientutils.ConvertLocalPatternToRegexp(addDepsParams.Pattern, addDepsParams.GetPatternType()))
	excludePathPattern := fspatterns.PrepareExcludePathPattern(addDepsParams.Exclusions, addDepsParams.GetPatternType(), addDepsParams.IsRecursive())
	patternRegex, err := regxp.Compile(addDepsParams.Pattern)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}

	paths, err := fspatterns.ListFiles(rootPath, addDepsParams.IsRecursive(), addDepsParams.IsIncludeDirs(), true, excludePathPattern)
	if err != nil {
		return nil, err
	}
	result := []string{}

	for _, path := range paths {
		matches, _, err := fspatterns.SearchPatterns(path, true, false, patternRegex)
		if err != nil {
			log.Error(err)
			continue
		}
		if len(matches) > 0 {
			result = append(result, path)
		}
	}
	return result, nil
}

func (badc *BuildAddDependenciesCommand) savePartialBuildInfo(dependencies []buildinfo.Dependency) error {
	log.Debug("Saving", strconv.Itoa(len(dependencies)), "dependencies.")
	populateFunc := func(partial *buildinfo.Partial) {
		partial.ModuleType = buildinfo.Generic
		partial.Dependencies = dependencies
		partial.ModuleId = badc.buildConfiguration.GetModule()
	}
	buildName, err := badc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := badc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	return utils.SavePartialBuildInfo(buildName, buildNumber, badc.buildConfiguration.GetProject(), populateFunc)
}

func convertFileInfoToDependencies(files map[string]*fileutils.FileDetails) []buildinfo.Dependency {
	var buildDependencies []buildinfo.Dependency
	for filePath, fileInfo := range files {
		dependency := buildinfo.Dependency{}
		dependency.Md5 = fileInfo.Checksum.Md5
		dependency.Sha1 = fileInfo.Checksum.Sha1
		filename, _ := fileutils.GetFileAndDirFromPath(filePath)
		dependency.Id = filename
		buildDependencies = append(buildDependencies, dependency)
	}
	return buildDependencies
}

func searchItems(spec *spec.SpecFiles, servicesManager artifactory.ArtifactoryServicesManager) (resultReader *content.ContentReader, err error) {
	temp := []*content.ContentReader{}
	var searchParams services.SearchParams
	var reader *content.ContentReader
	defer func() {
		for _, reader := range temp {
			e := reader.Close()
			if err == nil {
				err = e
			}
		}
	}()
	for i := 0; i < len(spec.Files); i++ {
		searchParams, err = utils.GetSearchParams(spec.Get(i))
		if err != nil {
			return
		}
		reader, err = servicesManager.SearchFiles(searchParams)
		if err != nil {
			return
		}
		temp = append(temp, reader)
	}
	resultReader, err = content.MergeReaders(temp, content.DefaultKey)
	return
}
