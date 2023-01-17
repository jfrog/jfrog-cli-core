package transferconfig

import (
	"fmt"
	"github.com/go-test/deep"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access"
	accessServices "github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"strings"
	"time"
)

type ConflictType string

const (
	minJFrogProjectsArtifactoryVersion              = "7.0.0"
	Repository                         ConflictType = "Repository"
	Project                            ConflictType = "Project"
	logFilePrefix                                   = "transfer-config-conflicts"
)

var filteredRepoKeys = []string{"Url", "password", "suppressPomConsistencyChecks"}

type TransferConfigMergeCommand struct {
	sourceServerDetails      *config.ServerDetails
	targetServerDetails      *config.ServerDetails
	includeReposPatterns     []string
	excludeReposPatterns     []string
	includeProjectsPatterns  []string
	excludeProjectsPatterns  []string
	sourceArtifactoryManager artifactory.ArtifactoryServicesManager
	targetArtifactoryManager artifactory.ArtifactoryServicesManager
	sourceAccessManager      access.AccessServicesManager
	targetAccessManager      access.AccessServicesManager
}

func NewTransferConfigMergeCommand(sourceServer, targetServer *config.ServerDetails) *TransferConfigMergeCommand {
	return &TransferConfigMergeCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer}
}

func (tcmc *TransferConfigMergeCommand) CommandName() string {
	return "rt_transfer_config_merge"
}

func (tcmc *TransferConfigMergeCommand) SetIncludeReposPatterns(includeReposPatterns []string) *TransferConfigMergeCommand {
	tcmc.includeReposPatterns = includeReposPatterns
	return tcmc
}

func (tcmc *TransferConfigMergeCommand) SetExcludeReposPatterns(excludeReposPatterns []string) *TransferConfigMergeCommand {
	tcmc.excludeReposPatterns = excludeReposPatterns
	return tcmc
}

func (tcmc *TransferConfigMergeCommand) SetIncludeProjectsPatterns(includeProjectsPatterns []string) *TransferConfigMergeCommand {
	tcmc.includeProjectsPatterns = includeProjectsPatterns
	return tcmc
}

func (tcmc *TransferConfigMergeCommand) SetExcludeProjectsPatterns(excludeProjectsPatterns []string) *TransferConfigMergeCommand {
	tcmc.excludeProjectsPatterns = excludeProjectsPatterns
	return tcmc
}

type Conflict struct {
	Type                ConflictType `json:"type,omitempty"`
	SourceName          string       `json:"source_name,omitempty"`
	TargetName          string       `json:"target_name,omitempty"`
	DifferentProperties string       `json:"different_properties,omitempty"`
}

func (tcmc *TransferConfigMergeCommand) Run() (csvPath string, err error) {
	log.Info(coreutils.PrintBoldTitle("========== Preparations =========="))
	projectsSupported, err := tcmc.initServiceManagersAndValidateServers()
	if err != nil {
		return
	}
	projectsToTransfer := []accessServices.Project{}
	conflicts := []Conflict{}
	if projectsSupported {
		log.Info(coreutils.PrintBoldTitle("========== Merging projects config =========="))
		projectsToTransfer, err = tcmc.mergeProjects(&conflicts)
		if err != nil {
			return
		}
	}

	log.Info(coreutils.PrintBoldTitle("========== Merging repositories config =========="))
	reposToTransfer, err := tcmc.mergeRepositories(&conflicts)
	if err != nil {
		return
	}

	if len(conflicts) != 0 {
		csvPath, err = commandUtils.CreateCSVFile(logFilePrefix, conflicts, time.Now())
		if err != nil {
			return
		}
		log.Info(fmt.Sprintf("We found %d conflicts when comparing the projects and repositories configuration between the source and target instances.\n"+
			"Please review the report available at %s", len(conflicts), csvPath) + "\n" +
			"You can either resolve the conflicts by manually modifying the configuration on the source or the target,\n" +
			"or exclude the transfer of the conflicting projects or repositories by adding options to this command.\n" +
			"Run 'jf rt transfer-config-merge -h' for more information.")
		return
	}

	log.Info("No Merge conflicts were found while comparing the source and target instances.")

	if len(projectsToTransfer) > 0 {
		log.Info(fmt.Sprintf("Transferring %d projects ...", len(projectsToTransfer)))
		err = tcmc.transferProjectsToTarget(projectsToTransfer)
		if err != nil {
			return
		}
	}

	if len(reposToTransfer) > 0 {
		log.Info(fmt.Sprintf("Transferring %d repositories ...", len(reposToTransfer)))
		err = tcmc.transferRepositoriesToTarget(reposToTransfer)
	}

	log.Info("Config transfer merge completed successfully!")
	return
}

func (tcmc *TransferConfigMergeCommand) initServiceManagersAndValidateServers() (projectsSupported bool, err error) {
	tcmc.sourceArtifactoryManager, err = utils.CreateServiceManager(tcmc.sourceServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	tcmc.targetArtifactoryManager, err = utils.CreateServiceManager(tcmc.targetServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	sourceArtifactoryVersion, err := validateMinVersionAndDifferentServers(tcmc.sourceArtifactoryManager, tcmc.sourceServerDetails, tcmc.targetServerDetails)
	if err != nil {
		return
	}

	// Check if JFrog Projects supported by Source Artifactory version
	versionErr := coreutils.ValidateMinimumVersion(coreutils.Projects, sourceArtifactoryVersion, minJFrogProjectsArtifactoryVersion)
	if versionErr != nil {
		// Projects not supported by Source Artifactory version
		return
	}

	projectsSupported = true
	sourceAccessManager, err := utils.CreateAccessServiceManager(tcmc.sourceServerDetails, false)
	if err != nil {
		return
	}
	tcmc.sourceAccessManager = *sourceAccessManager
	targetAccessManager, err := utils.CreateAccessServiceManager(tcmc.targetServerDetails, false)
	if err != nil {
		return
	}
	tcmc.targetAccessManager = *targetAccessManager

	if _, err = sourceAccessManager.Ping(); err != nil {
		err = errorutils.CheckErrorf("The source's access token is not valid. Please provide a valid access token by running the 'jf c edit'")
		return
	}
	if _, err = targetAccessManager.Ping(); err != nil {
		err = errorutils.CheckErrorf("The target's access token is not valid. Please provide a valid access token by running the 'jf c edit'")
	}
	return
}

func (tcmc *TransferConfigMergeCommand) mergeProjects(conflicts *[]Conflict) (projectsToTransfer []accessServices.Project, err error) {
	log.Info("Getting all Projects from the source ...")
	sourceProjects, err := tcmc.sourceAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	log.Info("Getting all Projects from the target ...")
	targetProjects, err := tcmc.targetAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	targetProjectsMapper := newProjectsMapper(targetProjects)
	includeExcludeFilter := &utils.IncludeExcludeFilter{
		IncludePatterns: tcmc.includeProjectsPatterns,
		ExcludePatterns: tcmc.excludeProjectsPatterns,
	}
	for _, sourceProject := range sourceProjects {
		// Check if repository is filtered out.
		var shouldIncludeProject bool
		shouldIncludeProject, err = includeExcludeFilter.ShouldIncludeItem(sourceProject.ProjectKey)
		if err != nil {
			return
		}
		if !shouldIncludeProject {
			continue
		}
		targetProjectWithSameKey := targetProjectsMapper.getProjectByKey(sourceProject.ProjectKey)
		targetProjectWithSameName := targetProjectsMapper.getProjectByName(sourceProject.DisplayName)

		if targetProjectWithSameKey == nil && targetProjectWithSameName == nil {
			// Project exists on source only, can be created on target
			projectsToTransfer = append(projectsToTransfer, sourceProject)
			continue
		}
		var conflict *Conflict
		if targetProjectWithSameKey != nil {
			// Project with the same projectKey exists on target
			conflict = compareProjects(sourceProject, *targetProjectWithSameKey)
			if conflict != nil {
				*conflicts = append(*conflicts, *conflict)
			}
		}
		if targetProjectWithSameName != nil && targetProjectWithSameName != targetProjectWithSameKey {
			// Project with the same display name but different projectKey exists on target
			conflict = compareProjects(sourceProject, *targetProjectWithSameName)
			if conflict != nil {
				*conflicts = append(*conflicts, *conflict)
			}
		}
	}
	return
}

func compareProjects(sourceProject, targetProject accessServices.Project) *Conflict {
	diff := compareInterfaces(sourceProject, targetProject)
	if diff == "" {
		return nil
	}
	return &Conflict{
		Type:                Project,
		SourceName:          fmt.Sprintf("%s(%s)", sourceProject.DisplayName, sourceProject.ProjectKey),
		TargetName:          fmt.Sprintf("%s(%s)", targetProject.DisplayName, targetProject.ProjectKey),
		DifferentProperties: diff,
	}
}

func (tcmc *TransferConfigMergeCommand) mergeRepositories(conflicts *[]Conflict) (reposToTransfer []string, err error) {
	sourceRepos, err := tcmc.sourceArtifactoryManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetRepos, err := tcmc.targetArtifactoryManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetReposMap := make(map[string]services.RepositoryDetails)
	for _, repo := range *targetRepos {
		targetReposMap[repo.Key] = repo
	}
	includeExcludeFilter := &utils.IncludeExcludeFilter{
		IncludePatterns: tcmc.includeReposPatterns,
		ExcludePatterns: tcmc.excludeReposPatterns,
	}
	for _, sourceRepo := range *sourceRepos {
		// Check if repository is filtered out.
		var shouldIncludeRepo bool
		shouldIncludeRepo, err = includeExcludeFilter.ShouldIncludeItem(sourceRepo.Key)
		if err != nil {
			return
		}
		if !shouldIncludeRepo {
			continue
		}
		if targetRepo, exists := targetReposMap[sourceRepo.Key]; exists {
			// The repository exists on target. We need to compare the repositories.
			diff := ""
			diff, err = tcmc.compareRepositories(sourceRepo, targetRepo)
			if err != nil {
				return
			}
			if diff != "" {
				// Conflicts found, adding to conflicts CSV
				*conflicts = append(*conflicts, Conflict{
					Type:                Repository,
					SourceName:          sourceRepo.Key,
					TargetName:          sourceRepo.Key,
					DifferentProperties: diff,
				})
			}
		} else {
			reposToTransfer = append(reposToTransfer, sourceRepo.Key)
		}
	}
	return
}

func (tcmc *TransferConfigMergeCommand) compareRepositories(sourceRepoBaseDetails, targetRepoBaseDetails services.RepositoryDetails) (diff string, err error) {
	// Compare basic repository details
	diff = compareInterfaces(sourceRepoBaseDetails, targetRepoBaseDetails, filteredRepoKeys...)
	if diff != "" {
		return
	}

	// The basic details are equal. Continuing to compare the full repository details.
	// Get full repo info from source and target
	var sourceRepoFullDetails interface{}
	err = tcmc.sourceArtifactoryManager.GetRepository(sourceRepoBaseDetails.Key, &sourceRepoFullDetails)
	if err != nil {
		return
	}
	var targetRepoFullDetails interface{}
	err = tcmc.targetArtifactoryManager.GetRepository(targetRepoBaseDetails.Key, &targetRepoFullDetails)
	if err != nil {
		return
	}
	diff = compareInterfaces(sourceRepoFullDetails, targetRepoFullDetails, filteredRepoKeys...)
	return
}

func compareInterfaces(first, second interface{}, filteredKeys ...string) string {
	diffs := deep.Equal(first, second)
	diffList := []string{}
	for _, diff := range diffs {
		key := strings.Split(diff, ":")[0]
		if slices.Contains(filteredKeys, strings.Split(key, ".")[0]) {
			continue
		}
		diffList = append(diffList, key)
	}
	return strings.Join(diffList, " ; ")
}

func (tcmc *TransferConfigMergeCommand) transferProjectsToTarget(reposToTransfer []accessServices.Project) (err error) {
	for _, project := range reposToTransfer {
		log.Info(fmt.Sprintf("Transferring project '%s' ...", project.DisplayName))
		if err = tcmc.targetAccessManager.CreateProject(accessServices.ProjectParams{ProjectDetails: project}); err != nil {
			return
		}
	}
	return
}

func (tcmc *TransferConfigMergeCommand) transferRepositoriesToTarget(reposToTransfer []string) (err error) {
	// Decrypt source Artifactory to get encrypted parameters
	var wasEncrypted bool
	if wasEncrypted, err = tcmc.sourceArtifactoryManager.DeactivateKeyEncryption(); err != nil {
		return
	}
	defer func() {
		if !wasEncrypted {
			return
		}
		activationErr := tcmc.sourceArtifactoryManager.ActivateKeyEncryption()
		if err == nil {
			err = activationErr
		}
	}()
	for _, repoKey := range reposToTransfer {
		var params interface{}
		err = tcmc.sourceArtifactoryManager.GetRepository(repoKey, &params)
		if err != nil {
			return
		}
		log.Info(fmt.Sprintf("Transferring the configuration of repository '%s' ...", repoKey))
		err = tcmc.targetArtifactoryManager.CreateRepositoryWithParams(params, repoKey)
		if err != nil {
			return
		}
	}

	return nil
}

type projectsMapper struct {
	byDisplayName map[string]*accessServices.Project
	byProjectKey  map[string]*accessServices.Project
}

func newProjectsMapper(targetProjects []accessServices.Project) *projectsMapper {
	byDisplayName := make(map[string]*accessServices.Project)
	byProjectKey := make(map[string]*accessServices.Project)
	for i, project := range targetProjects {
		byDisplayName[project.DisplayName] = &targetProjects[i]
		byProjectKey[project.ProjectKey] = &targetProjects[i]
	}
	return &projectsMapper{byDisplayName: byDisplayName, byProjectKey: byProjectKey}
}

func (p *projectsMapper) getProjectByName(displayName string) *accessServices.Project {
	return p.byDisplayName[displayName]
}

func (p *projectsMapper) getProjectByKey(projectKey string) *accessServices.Project {
	return p.byProjectKey[projectKey]
}
