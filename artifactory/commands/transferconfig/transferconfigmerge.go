package transferconfig

import (
	"fmt"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	accessServices "github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"reflect"
	"strings"
	"time"
)

type ConflictType string

const (
	minJFrogProjectsArtifactoryVersion              = "7.0.0"
	Repository                         ConflictType = "Repository"
	Project                            ConflictType = "Project"
)

type TransferConfigMergeCommand struct {
	sourceServerDetails  *config.ServerDetails
	targetServerDetails  *config.ServerDetails
	includeReposPatterns []string
	excludeReposPatterns []string
}

func NewTransferConfigMergeCommand(sourceServer, targetServer *config.ServerDetails) *TransferConfigMergeCommand {
	return &TransferConfigMergeCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer}
}

func (tcmc *TransferConfigMergeCommand) CommandName() string {
	return "rt_transfer_config_merge"
}

type Conflict struct {
	Type                ConflictType `json:"type,omitempty"`
	SourceName          string       `json:"source_name,omitempty"`
	TargetName          string       `json:"target_name,omitempty"`
	DifferentProperties string       `json:"different_properties,omitempty"`
}

func (tcmc *TransferConfigMergeCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 1/3 - Preparations ==========")))
	sourceServiceManager, err := utils.CreateServiceManager(tcmc.sourceServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	targetServiceManager, err := utils.CreateServiceManager(tcmc.targetServerDetails, -1, 0, false)
	if err != nil {
		return
	}
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	if err = validateMinVersionAndDifferentServers(sourceServiceManager, tcmc.sourceServerDetails, tcmc.targetServerDetails); err != nil {
		return
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 2/3 - Merging Repositories ==========")))
	conflicts := []Conflict{}
	err = tcmc.mergeRepositories(sourceServiceManager, targetServiceManager, &conflicts)
	if err != nil {
		return
	}

	sourceArtifactoryVersion, err := sourceServiceManager.GetVersion()
	if err != nil {
		return
	}
	err = coreutils.ValidateMinimumVersion(coreutils.Projects, sourceArtifactoryVersion, minJFrogProjectsArtifactoryVersion)
	if err == nil {
		log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 3/3 - Merging Projects ==========")))
		// check correctness of Authorization
		log.Info("Checking validation of your authorization methods..")
		if _, err = sourceServiceManager.Ping(); err != nil {
			err = errorutils.CheckErrorf("The source's access token is not valid. Please provide a valid access token.")
			return
		}
		if _, err = targetServiceManager.Ping(); err != nil {
			err = errorutils.CheckErrorf("The target's access token is not valid. Please provide a valid access token.")
			return
		}
		err = tcmc.mergeProjects(&conflicts)
		if err != nil {
			return
		}
	}

	if len(conflicts) == 0 {
		var path string
		path, err = commandUtils.CreateCSVFile("transfer-config-conflicts", conflicts, time.Now())
		if err != nil {
			return
		}
		log.Info(fmt.Sprintf("We found %d conflicts when comparing the source and target instances. Please review the following report available at %s", len(conflicts), path))
	} else {
		log.Info("No conflicts were found when comparing the source and target instances.")
	}
	return
}

func (tcmc *TransferConfigMergeCommand) mergeProjects(conflicts *[]Conflict) (err error) {
	log.Info("Getting all Projects ...")
	sourceServiceAccessManager, err := utils.CreateAccessServiceManager(tcmc.sourceServerDetails, false)
	sourceProjects, err := sourceServiceAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	targetServiceAccessManager, err := utils.CreateAccessServiceManager(tcmc.targetServerDetails, false)
	targetProjects, err := targetServiceAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	targetProjectsMapper := NewProjectsMapper(targetProjects)
	for _, sourceProject := range sourceProjects {
		targetProjectWithSameKey := targetProjectsMapper.GetProjectByKey(sourceProject.ProjectKey)
		targetProjectWithSameName := targetProjectsMapper.GetProjectByName(sourceProject.DisplayName)

		if targetProjectWithSameKey == nil && targetProjectWithSameName == nil {
			// Project exists on source only, can be created on target
			if err = targetServiceAccessManager.CreateProject(accessServices.ProjectParams{ProjectDetails: sourceProject}); err != nil {
				return
			}
			continue
		}
		if targetProjectWithSameKey != nil {
			// Project with the same projectKey exists on target
			var conflict *Conflict
			conflict, err = compareProjects(sourceProject, *targetProjectWithSameKey)
			if err != nil {
				return
			}
			if conflict != nil {
				*conflicts = append(*conflicts, *conflict)
			}
		}
		if targetProjectWithSameName != nil && targetProjectWithSameName != targetProjectWithSameKey {
			// // Project with the same Display name but different projectKey exists on target
			var conflict *Conflict
			conflict, err = compareProjects(sourceProject, *targetProjectWithSameName)
			if err != nil {
				return
			}
			if conflict != nil {
				*conflicts = append(*conflicts, *conflict)
			}
		}
	}
	return
}

func compareProjects(sourceProject, targetProject accessServices.Project) (*Conflict, error) {
	diff, err := compareInterfaces(sourceProject, targetProject)
	if err != nil {
		return nil, err
	}
	return &Conflict{
		Type:                Project,
		SourceName:          fmt.Sprintf("%s(%s)", sourceProject.DisplayName, sourceProject.ProjectKey),
		TargetName:          fmt.Sprintf("%s(%s)", targetProject.DisplayName, targetProject.ProjectKey),
		DifferentProperties: strings.Join(diff, ","),
	}, nil
}

func (tcmc *TransferConfigMergeCommand) mergeRepositories(sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager, conflicts *[]Conflict) (err error) {
	log.Info("Getting all repositories ...")
	sourceRepos, err := sourceServiceManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetRepos, err := targetServiceManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetReposMap := make(map[string]services.RepositoryDetails)
	for _, repo := range *targetRepos {
		targetReposMap[repo.Key] = repo
	}
	reposToTransfer := []string{}
	for _, sourceRepo := range *sourceRepos {
		// Check if repository is filtered
		repoFilter := &utils.RepositoryFilter{
			IncludePatterns: tcmc.includeReposPatterns,
			ExcludePatterns: tcmc.excludeReposPatterns,
		}
		var shouldIncludeRepo bool
		shouldIncludeRepo, err = repoFilter.ShouldIncludeRepository(sourceRepo.Key)
		if err != nil {
			return
		}
		if !shouldIncludeRepo {
			continue
		}
		if targetRepo, exists := targetReposMap[sourceRepo.Key]; exists {
			// The repository exists on target. We need to compare the repositories.
			var reposDiff []string
			reposDiff, err = compareRepositories(sourceRepo, targetRepo, sourceServiceManager, targetServiceManager)
			if err != nil {
				return
			}
			if len(reposDiff) != 0 {
				// Conflicts found, adding to conflicts CSV
				*conflicts = append(*conflicts, Conflict{
					Type:                Repository,
					SourceName:          sourceRepo.Key,
					TargetName:          sourceRepo.Key,
					DifferentProperties: strings.Join(reposDiff, ","),
				})
			}
		} else {
			reposToTransfer = append(reposToTransfer, sourceRepo.Key)
		}
	}
	if len(reposToTransfer) > 0 {
		err = transferRepositoriesToTarget(reposToTransfer, sourceServiceManager, targetServiceManager)
	}
	return
}

func compareRepositories(sourceRepoBaseDetails, targetRepoBaseDetails services.RepositoryDetails, sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) (diff []string, err error) {
	// Compare basic repository details
	diff, err = compareInterfaces(sourceRepoBaseDetails, targetRepoBaseDetails, "Url")
	if err != nil && len(diff) != 0 {
		return
	}

	// The basic details are equal. Continuing to compare the full repository details.
	// Get full repo info from source and target
	var sourceRepoFullDetails interface{}
	err = sourceServiceManager.GetRepository(sourceRepoBaseDetails.Key, &sourceRepoFullDetails)
	if err != nil {
		return
	}
	var targetRepoFullDetails interface{}
	err = targetServiceManager.GetRepository(targetRepoBaseDetails.Key, &targetRepoFullDetails)
	if err != nil {
		return
	}
	diff, err = compareInterfaces(sourceRepoFullDetails, targetRepoFullDetails, "password", "suppressPomConsistencyChecks")
	return
}

func compareInterfaces(first, second interface{}, filteredKeys ...string) (diff []string, err error) {
	firstMap, err := coreutils.InterfaceToMap(first)
	if err != nil {
		return
	}
	secondMap, err := coreutils.InterfaceToMap(second)
	if err != nil {
		return
	}
	for key, firstValue := range firstMap {
		if slices.Contains(filteredKeys, key) {
			// Key should be filtered out
			continue
		}
		if secondValue, ok := secondMap[key]; ok {
			// Keys only compared when exists on both interfaces
			if !reflect.DeepEqual(firstValue, secondValue) {
				diff = append(diff, key)
			}
		}
	}
	return
}

func transferRepositoriesToTarget(reposToTransfer []string, sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) (err error) {
	// Decrypt source artifactory to get encrypted parameters
	var wasEncrypted bool
	if wasEncrypted, err = sourceServiceManager.DeactivateKeyEncryption(); err != nil {
		return
	}
	defer func() {
		if !wasEncrypted {
			return
		}
		activationErr := sourceServiceManager.ActivateKeyEncryption()
		if err == nil {
			err = activationErr
		}
	}()
	for _, repoKey := range reposToTransfer {
		var params interface{}
		err = sourceServiceManager.GetRepository(repoKey, &params)
		if err != nil {
			return
		}
		err = targetServiceManager.CreateRepositoryWithParams(params, repoKey)
		if err != nil {
			return
		}
	}

	return nil
}

type ProjectsMapper struct {
	byDisplayName map[string]*accessServices.Project
	byProjectKey  map[string]*accessServices.Project
}

func NewProjectsMapper(targetProjects []accessServices.Project) *ProjectsMapper {
	byDisplayName := make(map[string]*accessServices.Project)
	byProjectKey := make(map[string]*accessServices.Project)
	for i, project := range targetProjects {
		byDisplayName[project.DisplayName] = &targetProjects[i]
		byProjectKey[project.ProjectKey] = &targetProjects[i]
	}
	return &ProjectsMapper{byDisplayName: byDisplayName, byProjectKey: byProjectKey}
}

func (p *ProjectsMapper) GetProjectByName(displayName string) *accessServices.Project {
	return p.byDisplayName[displayName]
}

func (p *ProjectsMapper) GetProjectByKey(projectKey string) *accessServices.Project {
	return p.byProjectKey[projectKey]
}
