package transferconfigmerge

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	accessServices "github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

type ConflictType string

const (
	Repository    ConflictType = "Repository"
	Project       ConflictType = "Project"
	logFilePrefix              = "transfer-config-conflicts"
)

// Repository key that should be filtered when comparing repositories (all must be lowercase)
var filteredRepoKeys = []string{"url", "password", "suppresspomconsistencychecks", "description", "gitregistryurl", "cargointernalindex"}

type TransferConfigMergeCommand struct {
	commandsUtils.TransferConfigBase
	includeProjectsPatterns []string
	excludeProjectsPatterns []string
}

func NewTransferConfigMergeCommand(sourceServer, targetServer *config.ServerDetails) *TransferConfigMergeCommand {
	return &TransferConfigMergeCommand{TransferConfigBase: *commandsUtils.NewTransferConfigBase(sourceServer, targetServer)}
}

func (tcmc *TransferConfigMergeCommand) CommandName() string {
	return "rt_transfer_config_merge"
}

func (tcmc *TransferConfigMergeCommand) SetIncludeProjectsPatterns(includeProjectsPatterns []string) *TransferConfigMergeCommand {
	tcmc.includeProjectsPatterns = includeProjectsPatterns
	return tcmc
}

func (tcmc *TransferConfigMergeCommand) SetExcludeProjectsPatterns(excludeProjectsPatterns []string) *TransferConfigMergeCommand {
	tcmc.excludeProjectsPatterns = excludeProjectsPatterns
	return tcmc
}

type mergeEntities struct {
	projectsToTransfer []accessServices.Project
	reposToTransfer    map[utils.RepoType][]string
}

type Conflict struct {
	Type                ConflictType `json:"type,omitempty"`
	SourceName          string       `json:"source_name,omitempty"`
	TargetName          string       `json:"target_name,omitempty"`
	DifferentProperties string       `json:"different_properties,omitempty"`
}

func (tcmc *TransferConfigMergeCommand) Run() (csvPath string, err error) {
	tcmc.LogTitle("Preparations")
	projectsSupported, err := tcmc.initServiceManagersAndValidateServers()
	if err != nil {
		return
	}

	var mergeEntities mergeEntities
	mergeEntities, csvPath, err = tcmc.mergeEntities(projectsSupported)
	if err != nil {
		return
	}

	if err = tcmc.transferEntities(mergeEntities); err != nil {
		return
	}

	log.Info("Config transfer merge completed successfully!")
	tcmc.LogIfFederatedMemberRemoved()
	return
}

func (tcmc *TransferConfigMergeCommand) initServiceManagersAndValidateServers() (projectsSupported bool, err error) {
	if err = tcmc.CreateServiceManagers(false); err != nil {
		return
	}
	// Make sure source and target Artifactory URLs are different and the source Artifactory version is sufficient.
	err = tcmc.ValidateDifferentServers()
	if err != nil {
		return
	}
	sourceArtifactoryVersion, err := tcmc.SourceArtifactoryManager.GetVersion()
	if err != nil {
		return
	}
	// Check if JFrog Projects supported by Source Artifactory version
	versionErr := coreutils.ValidateMinimumVersion(coreutils.Projects, sourceArtifactoryVersion, commandsUtils.MinJFrogProjectsArtifactoryVersion)
	if versionErr != nil {
		// Projects not supported by Source Artifactory version
		return
	}

	projectsSupported = true

	if err = tcmc.ValidateAccessServerConnection(tcmc.SourceServerDetails, tcmc.SourceAccessManager); err != nil {
		return
	}
	if err = tcmc.ValidateAccessServerConnection(tcmc.TargetServerDetails, tcmc.TargetAccessManager); err != nil {
		return
	}

	return
}

func (tcmc *TransferConfigMergeCommand) mergeEntities(projectsSupported bool) (mergeEntities mergeEntities, csvPath string, err error) {
	conflicts := []Conflict{}
	if projectsSupported {
		tcmc.LogTitle("Merging projects config")
		mergeEntities.projectsToTransfer, err = tcmc.mergeProjects(&conflicts)
		if err != nil {
			return
		}
	}

	tcmc.LogTitle("Merging repositories config")
	mergeEntities.reposToTransfer, err = tcmc.mergeRepositories(&conflicts)
	if err != nil {
		return
	}

	if len(conflicts) != 0 {
		csvPath, err = commandsUtils.CreateCSVFile(logFilePrefix, conflicts, time.Now())
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
	return
}

func (tcmc *TransferConfigMergeCommand) transferEntities(mergeEntities mergeEntities) (err error) {
	if len(mergeEntities.projectsToTransfer) > 0 {
		tcmc.LogTitle("Transferring projects")
		err = tcmc.transferProjectsToTarget(mergeEntities.projectsToTransfer)
		if err != nil {
			return
		}
	}

	tcmc.LogTitle("Transferring repositories")
	var remoteRepositories []interface{}
	if len(mergeEntities.reposToTransfer[utils.Remote]) > 0 {
		remoteRepositories, err = tcmc.decryptAndGetAllRemoteRepositories(mergeEntities.reposToTransfer[utils.Remote])
		if err != nil {
			return
		}
	}

	return tcmc.TransferRepositoriesToTarget(mergeEntities.reposToTransfer, remoteRepositories)
}

func (tcmc *TransferConfigMergeCommand) mergeProjects(conflicts *[]Conflict) (projectsToTransfer []accessServices.Project, err error) {
	log.Info("Getting all Projects from the source ...")
	sourceProjects, err := tcmc.SourceAccessManager.GetAllProjects()
	if err != nil {
		return
	}
	log.Info("Getting all Projects from the target ...")
	targetProjects, err := tcmc.TargetAccessManager.GetAllProjects()
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
			conflict, err = compareProjects(sourceProject, *targetProjectWithSameKey)
			if err != nil {
				return
			}
			if conflict != nil {
				*conflicts = append(*conflicts, *conflict)
			}
		}
		if targetProjectWithSameName != nil && targetProjectWithSameName != targetProjectWithSameKey {
			// Project with the same display name but different projectKey exists on target
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
	if err != nil || diff == "" {
		return nil, err
	}
	return &Conflict{
		Type:                Project,
		SourceName:          fmt.Sprintf("%s(%s)", sourceProject.DisplayName, sourceProject.ProjectKey),
		TargetName:          fmt.Sprintf("%s(%s)", targetProject.DisplayName, targetProject.ProjectKey),
		DifferentProperties: diff,
	}, nil
}

func (tcmc *TransferConfigMergeCommand) mergeRepositories(conflicts *[]Conflict) (reposToTransfer map[utils.RepoType][]string, err error) {
	reposToTransfer = make(map[utils.RepoType][]string)
	sourceRepos, err := tcmc.SourceArtifactoryManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetRepos, err := tcmc.TargetArtifactoryManager.GetAllRepositories()
	if err != nil {
		return
	}
	targetReposMap := make(map[string]services.RepositoryDetails)
	for _, repo := range *targetRepos {
		targetReposMap[repo.Key] = repo
	}
	includeExcludeFilter := tcmc.GetRepoFilter()
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
			var diff string
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
			repoType := utils.RepoTypeFromString(sourceRepo.Type)
			reposToTransfer[repoType] = append(reposToTransfer[repoType], sourceRepo.Key)
		}
	}
	return
}

func (tcmc *TransferConfigMergeCommand) compareRepositories(sourceRepoBaseDetails, targetRepoBaseDetails services.RepositoryDetails) (diff string, err error) {
	// Compare basic repository details
	diff, err = compareInterfaces(sourceRepoBaseDetails, targetRepoBaseDetails, filteredRepoKeys...)
	if err != nil || diff != "" {
		return
	}

	// The basic details are equal. Continuing to compare the full repository details.
	// Get full repo info from source and target
	var sourceRepoFullDetails interface{}
	err = tcmc.SourceArtifactoryManager.GetRepository(sourceRepoBaseDetails.Key, &sourceRepoFullDetails)
	if err != nil {
		return
	}
	var targetRepoFullDetails interface{}
	err = tcmc.TargetArtifactoryManager.GetRepository(targetRepoBaseDetails.Key, &targetRepoFullDetails)
	if err != nil {
		return
	}
	diff, err = compareInterfaces(sourceRepoFullDetails, targetRepoFullDetails, filteredRepoKeys...)
	return
}

func compareInterfaces(first, second interface{}, filteredKeys ...string) (diff string, err error) {
	firstMap, err := commandsUtils.InterfaceToMap(first)
	if err != nil {
		return
	}
	secondMap, err := commandsUtils.InterfaceToMap(second)
	if err != nil {
		return
	}
	diffList := []string{}
	for key, firstValue := range firstMap {
		if slices.Contains(filteredKeys, strings.ToLower(key)) {
			// Key should be filtered out
			continue
		}
		if secondValue, ok := secondMap[key]; ok {
			// Keys are only compared when exiting on both interfaces.
			if !reflect.DeepEqual(firstValue, secondValue) {
				diffList = append(diffList, key)
			}
		}
	}
	diff = strings.Join(diffList, "; ")
	return
}

func (tcmc *TransferConfigMergeCommand) transferProjectsToTarget(reposToTransfer []accessServices.Project) (err error) {
	for _, project := range reposToTransfer {
		log.Info(fmt.Sprintf("Transferring project '%s' ...", project.DisplayName))
		if err = tcmc.TargetAccessManager.CreateProject(accessServices.ProjectParams{ProjectDetails: project}); err != nil {
			return
		}
	}
	return
}

func (tcmc *TransferConfigMergeCommand) decryptAndGetAllRemoteRepositories(remoteRepositoryNames []string) (remoteRepositories []interface{}, err error) {
	// Decrypt source Artifactory to get remote repositories with raw text passwords
	reactivateKeyEncryption, err := tcmc.DeactivateKeyEncryption()
	if err != nil {
		return
	}
	defer func() {
		if reactivationErr := reactivateKeyEncryption(); err == nil {
			err = reactivationErr
		}
	}()
	return tcmc.GetAllRemoteRepositories(remoteRepositoryNames)
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
