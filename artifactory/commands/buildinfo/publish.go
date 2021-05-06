package buildinfo

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"sort"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type BuildPublishCommand struct {
	buildConfiguration *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	config             *buildinfo.Configuration
	detailedSummary    bool
	summary            *services.BuildPublishSummary
}

func NewBuildPublishCommand() *BuildPublishCommand {
	return &BuildPublishCommand{}
}

func (bpc *BuildPublishCommand) SetConfig(config *buildinfo.Configuration) *BuildPublishCommand {
	bpc.config = config
	return bpc
}

func (bpc *BuildPublishCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildPublishCommand {
	bpc.serverDetails = serverDetails
	return bpc
}

func (bpc *BuildPublishCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildPublishCommand {
	bpc.buildConfiguration = buildConfiguration
	return bpc
}

func (bpc *BuildPublishCommand) SetSummary(summary *services.BuildPublishSummary) *BuildPublishCommand {
	bpc.summary = summary
	return bpc
}

func (bpc *BuildPublishCommand) GetSummary() *services.BuildPublishSummary {
	return bpc.summary
}

func (bpc *BuildPublishCommand) SetDetailedSummary(detailedSummary bool) *BuildPublishCommand {
	bpc.detailedSummary = detailedSummary
	return bpc
}

func (bpc *BuildPublishCommand) IsDetailedSummary() bool {
	return bpc.detailedSummary
}

func (bpc *BuildPublishCommand) CommandName() string {
	return "rt_build_publish"
}

func (bpc *BuildPublishCommand) ServerDetails() (*config.ServerDetails, error) {
	return bpc.serverDetails, nil
}

func (bpc *BuildPublishCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(bpc.serverDetails, bpc.config.DryRun)
	if err != nil {
		return err
	}

	buildInfo, err := bpc.createBuildInfoFromPartials()
	if err != nil {
		return err
	}

	generatedBuildsInfo, err := utils.GetGeneratedBuildsInfo(bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber, bpc.buildConfiguration.Project)
	if err != nil {
		return err
	}

	for _, v := range generatedBuildsInfo {
		buildInfo.Append(v)
	}
	summary, err := servicesManager.PublishBuildInfo(buildInfo, bpc.buildConfiguration.Project)
	if bpc.IsDetailedSummary() {
		bpc.SetSummary(summary)
	}
	if err != nil {
		return err
	}
	if !bpc.config.DryRun {
		return utils.RemoveBuildDir(bpc.buildConfiguration.BuildName, bpc.buildConfiguration.BuildNumber, bpc.buildConfiguration.Project)
	}
	return nil
}

func (bpc *BuildPublishCommand) createBuildInfoFromPartials() (*buildinfo.BuildInfo, error) {
	buildName := bpc.buildConfiguration.BuildName
	buildNumber := bpc.buildConfiguration.BuildNumber
	projectKey := bpc.buildConfiguration.Project
	partials, err := utils.ReadPartialBuildInfoFiles(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, err
	}
	sort.Sort(partials)

	buildInfo := buildinfo.New()
	buildInfo.SetAgentName(coreutils.GetCliUserAgentName())
	buildInfo.SetAgentVersion(coreutils.GetCliUserAgentVersion())
	buildInfo.SetBuildAgentVersion(coreutils.GetClientAgentVersion())
	buildInfo.Name = buildName
	buildInfo.Number = buildNumber
	buildGeneralDetails, err := utils.ReadBuildInfoGeneralDetails(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, err
	}
	buildInfo.Started = buildGeneralDetails.Timestamp.Format(buildinfo.TimeFormat)
	modules, env, vcsList, issues, err := extractBuildInfoData(partials, bpc.config.IncludeFilter(), bpc.config.ExcludeFilter())
	if err != nil {
		return nil, err
	}
	if len(env) != 0 {
		buildInfo.Properties = env
	}
	buildInfo.ArtifactoryPrincipal = bpc.serverDetails.User
	buildInfo.BuildUrl = bpc.config.BuildUrl
	for _, vcs := range vcsList {
		buildInfo.VcsList = append(buildInfo.VcsList, vcs)
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

func extractBuildInfoData(partials buildinfo.Partials, includeFilter, excludeFilter buildinfo.Filter) ([]buildinfo.Module, buildinfo.Env, []buildinfo.Vcs, buildinfo.Issues, error) {
	var vcs []buildinfo.Vcs
	var issues buildinfo.Issues
	env := make(map[string]string)
	partialModules := make(map[string]partialModule)
	issuesMap := make(map[string]*buildinfo.AffectedIssue)
	for _, partial := range partials {
		switch {
		case partial.Artifacts != nil:
			for _, artifact := range partial.Artifacts {
				addArtifactToPartialModule(artifact, partial, partialModules)
			}
		case partial.Dependencies != nil:
			for _, dependency := range partial.Dependencies {
				addDependencyToPartialModule(dependency, partial, partialModules)
			}
		case partial.VcsList != nil:
			for _, partialVcs := range partial.VcsList {
				vcs = append(vcs, partialVcs)
			}
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
		case partial.ModuleType == buildinfo.Build:
			partialModules[partial.ModuleId] = partialModule{
				moduleType: partial.ModuleType,
				checksum:   partial.Checksum,
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
		modules = append(modules, *createModule(moduleId, singlePartialModule.moduleType, singlePartialModule.checksum, moduleArtifacts, moduleDependencies))
	}
	return modules
}

func issuesMapToArray(issues buildinfo.Issues, issuesMap map[string]*buildinfo.AffectedIssue) buildinfo.Issues {
	for _, issue := range issuesMap {
		issues.AffectedIssues = append(issues.AffectedIssues, *issue)
	}
	return issues
}

func addDependencyToPartialModule(dependency buildinfo.Dependency, partial *buildinfo.Partial, partialModules map[string]partialModule) {
	// init map if needed
	moduleId := partial.ModuleId
	if partialModules[moduleId].dependencies == nil {
		partialModules[moduleId] = partialModule{
			artifacts:    partialModules[moduleId].artifacts,
			dependencies: make(map[string]buildinfo.Dependency),
			moduleType:   partial.ModuleType,
		}
	}
	key := fmt.Sprintf("%s-%s-%s-%s", dependency.Id, dependency.Sha1, dependency.Md5, dependency.Scopes)
	partialModules[moduleId].dependencies[key] = dependency
}

func addArtifactToPartialModule(artifact buildinfo.Artifact, partial *buildinfo.Partial, partialModules map[string]partialModule) {
	// init map if needed
	moduleId := partial.ModuleId
	if partialModules[moduleId].artifacts == nil {
		partialModules[moduleId] = partialModule{
			artifacts:    make(map[string]buildinfo.Artifact),
			dependencies: partialModules[moduleId].dependencies,
			moduleType:   partial.ModuleType,
		}
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

func createModule(moduleId string, moduleType buildinfo.ModuleType, checksum *buildinfo.Checksum, artifacts []buildinfo.Artifact, dependencies []buildinfo.Dependency) *buildinfo.Module {
	module := createDefaultModule(moduleId)
	module.Type = moduleType
	module.Checksum = checksum
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
	moduleType   buildinfo.ModuleType
	artifacts    map[string]buildinfo.Artifact
	dependencies map[string]buildinfo.Dependency
	checksum     *buildinfo.Checksum
}
