package transferconfig

import (
	"fmt"
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
	sourceServerDetails      *config.ServerDetails
	targetServerDetails      *config.ServerDetails
	includeReposPatterns     []string
	excludeReposPatterns     []string
	projectsSupported        bool
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

type Conflict struct {
	Type                ConflictType `json:"type,omitempty"`
	SourceName          string       `json:"source_name,omitempty"`
	TargetName          string       `json:"target_name,omitempty"`
	DifferentProperties string       `json:"different_properties,omitempty"`
}

func (tcmc *TransferConfigMergeCommand) Run() (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 1/3 - Preparations ==========")))
	err = tcmc.initServiceManagersAndValidateServers()
	if err != nil {
		return
	}

	conflicts := []Conflict{}
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 2/3 - Merging Repositories ==========")))
	err = tcmc.mergeRepositories(&conflicts)
	if err != nil {
		return
	}

	if tcmc.projectsSupported {
		log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 3/3 - Merging Projects ==========")))
		// check correctness of Authorization
		if err = tcmc.mergeProjects(&conflicts); err != nil {
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
		log.Info("No Merge conflicts were found while comparing the source and target instances.")
	}
	return
}

func (tcmc *TransferConfigMergeCommand) initServiceManagersAndValidateServers() error {
	var err error
	tcmc.sourceArtifactoryManager, err = utils.CreateServiceManager(tcmc.sourceServerDetails, -1, 0, false)
	if err != nil {
		return err
	}
	tcmc.sourceArtifactoryManager, err = utils.CreateServiceManager(tcmc.targetServerDetails, -1, 0, false)
	if err != nil {
		return err
	}
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	sourceArtifactoryVersion, err := validateMinVersionAndDifferentServers(tcmc.sourceArtifactoryManager, tcmc.sourceServerDetails, tcmc.targetServerDetails)
	if err != nil {
		return err
	}

	// Check if JFrog Projects supported by Source Artifactory version
	err = coreutils.ValidateMinimumVersion(coreutils.Projects, sourceArtifactoryVersion, minJFrogProjectsArtifactoryVersion)
	if err == nil {
		tcmc.projectsSupported = true
		sourceAccessManager, err := utils.CreateAccessServiceManager(tcmc.sourceServerDetails, false)
		if err != nil {
			return err
		}
		targetAccessManager, err := utils.CreateAccessServiceManager(tcmc.targetServerDetails, false)
		if err != nil {
			return err
		}

		log.Info("Checking validation of your authorization methods..")
		if _, err = sourceAccessManager.Ping(); err != nil {
			err = errorutils.CheckErrorf("The source's access token is not valid. Please provide a valid access token.")
			return err
		}
		if _, err = targetAccessManager.Ping(); err != nil {
			err = errorutils.CheckErrorf("The target's access token is not valid. Please provide a valid access token.")
			return err
		}
		tcmc.sourceAccessManager = *sourceAccessManager
		tcmc.targetAccessManager = *targetAccessManager
	}
	return nil
}
func (tcmc *TransferConfigMergeCommand) mergeProjects(conflicts *[]Conflict) (err error) {
	log.Info("Getting all Projects ...")
	sourceProjects, err := tcmc.sourceAccessManager.GetAllProjects()
	if err != nil {
		return
	}

	targetProjects, err := tcmc.targetAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	targetProjectsMapper := NewProjectsMapper(targetProjects)
	for _, sourceProject := range sourceProjects {
		targetProjectWithSameKey := targetProjectsMapper.GetProjectByKey(sourceProject.ProjectKey)
		targetProjectWithSameName := targetProjectsMapper.GetProjectByName(sourceProject.DisplayName)

		if targetProjectWithSameKey == nil && targetProjectWithSameName == nil {
			// Project exists on source only, can be created on target
			if err = tcmc.targetAccessManager.CreateProject(accessServices.ProjectParams{ProjectDetails: sourceProject}); err != nil {
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

func (tcmc *TransferConfigMergeCommand) mergeRepositories(conflicts *[]Conflict) (err error) {
	log.Info("Getting all repositories ...")
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
			reposDiff, err = tcmc.compareRepositories(sourceRepo, targetRepo)
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
		log.Info(fmt.Sprintf("Transferring %d repositories ...", len(reposToTransfer)))
		err = tcmc.transferRepositoriesToTarget(reposToTransfer)
	}
	return
}

func (tcmc *TransferConfigMergeCommand) compareRepositories(sourceRepoBaseDetails, targetRepoBaseDetails services.RepositoryDetails) (diff []string, err error) {
	// Compare basic repository details
	diff, err = compareInterfaces(sourceRepoBaseDetails, targetRepoBaseDetails, "Url")
	if err != nil && len(diff) != 0 {
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

func (tcmc *TransferConfigMergeCommand) transferRepositoriesToTarget(reposToTransfer []string) (err error) {
	// Decrypt source artifactory to get encrypted parameters
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
		err = tcmc.targetArtifactoryManager.CreateRepositoryWithParams(params, repoKey)
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
