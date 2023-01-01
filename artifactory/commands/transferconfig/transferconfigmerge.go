package transferconfig

import (
	"fmt"
	"github.com/gocarina/gocsv"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	loguitils "github.com/jfrog/jfrog-cli-core/v2/utils/log"
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

const (
	minArtifactoryMergeVersion = "7.0.0"
)

type ProjectConflict struct {
	SourceProjectName string `json:"source_project_name,omitempty"`
	TargetProjectName string `json:"target_project_name,omitempty"`
	SourceProjectKey  string `json:"source_project_key,omitempty"`
	TargetProjectKey  string `json:"target_project_key,omitempty"`
	DifferentProperty string `json:"different_property,omitempty"`
}

type RepositoryConflict struct {
	RepositoryName    string `json:"repository_name,omitempty"`
	DifferentProperty string `json:"different_property,omitempty"`
}

func (tcc *TransferConfigCommand) newConflict(sourceProjectName, targetProjectName, sourceProjectKey, targetProjectKey, differentProperty string) ProjectConflict {
	conflict := ProjectConflict{SourceProjectName: sourceProjectName, TargetProjectName: targetProjectName, SourceProjectKey: sourceProjectKey, TargetProjectKey: targetProjectKey, DifferentProperty: differentProperty}
	return conflict
}

func (tcc *TransferConfigCommand) RunMergeCommand(sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 1/3 - Preparations ==========")))
	sourceArtifactoryVersion, err := sourceServiceManager.GetVersion()
	if err != nil {
		return err
	}
	// Make sure that the source and target Artifactory servers are different and that the target Artifactory is empty
	transferProjects, err := tcc.validateMergeArtifactoryServers(targetServiceManager, sourceArtifactoryVersion, minArtifactoryMergeVersion)
	if err != nil {
		return
	}
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 2/3 - Transferring Projects ==========")))
	if !transferProjects {
		log.Info(fmt.Sprintf("Your source version is %s, there is no Projects in this version we will transfer your repositories", sourceArtifactoryVersion))
	} else {
		sourceServiceAccessManager, err := utils.CreateAccessServiceManager(tcc.sourceServerDetails, tcc.dryRun)
		sourceProjects, err := sourceServiceAccessManager.GetAllProjects()
		if err != nil {
			return err
		}
		targetServiceAccessManager, err := utils.CreateAccessServiceManager(tcc.targetServerDetails, tcc.dryRun)
		targerprojects, err := targetServiceAccessManager.GetAllProjects()
		if err != nil {
			return err
		}

		var projectConflicts []ProjectConflict
		conflict := false
		var isConflict bool
		for _, sourceProject := range sourceProjects {
			for _, targetProject := range targerprojects {

				projectConflicts, isConflict = tcc.validateProjectConflict(sourceProject, targetProject, projectConflicts)
				conflict = conflict || isConflict
			}
			if !conflict {
				err := targetServiceAccessManager.CreateProject(accessServices.ProjectParams{ProjectDetails: sourceProject})
				if err != nil {
					return err
				}
			}
		}
		csvPath, err := tcc.createConflictsCSVSummary(projectConflicts, time.Now())
		if err != nil {
			log.Error("Couldn't create the long properties CSV file", err)
			return err
		}

		log.Info(fmt.Sprintf("Founded %d projectConflicts projects between the source service and the target, check in csv file we created for you in %s", len(projectConflicts), csvPath))
	}
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 3/3 - Transferring Repositories ==========")))
	err = tcc.mergeRepositories(sourceServiceManager, targetServiceManager)

	return err
}

func (tcc *TransferConfigCommand) validateProjectConflict(sourceProject, targetProject accessServices.Project, conflicts []ProjectConflict) ([]ProjectConflict, bool) {
	result := false
	if sourceProject.ProjectKey == targetProject.ProjectKey || sourceProject.DisplayName == targetProject.DisplayName {
		s := ""
		result = true
		if sourceProject.DisplayName != targetProject.DisplayName {
			s = tcc.addToDifferentProperty(s, "Display Name")
		}
		if sourceProject.ProjectKey != targetProject.ProjectKey {
			s = tcc.addToDifferentProperty(s, "Project Key")
		}
		if result {
			if sourceProject.Description != targetProject.Description {
				s = tcc.addToDifferentProperty(s, "Description")
			}
			if sourceProject.StorageQuotaBytes != targetProject.StorageQuotaBytes {
				s = tcc.addToDifferentProperty(s, "Storage Quota Bytes")
			}
			if sourceProject.SoftLimit != nil && targetProject.SoftLimit != nil {
				if *sourceProject.SoftLimit != *targetProject.SoftLimit {
					s = tcc.addToDifferentProperty(s, "Soft Limit")
				}
			}
			if sourceProject.SoftLimit == nil || targetProject.SoftLimit == nil {
				if sourceProject.SoftLimit != nil || targetProject.SoftLimit != nil {
					s = tcc.addToDifferentProperty(s, "Soft Limit")
				}
			}
			if !tcc.checkIfSameAdminPrivilige(sourceProject.AdminPrivileges, targetProject.AdminPrivileges) {
				s = tcc.addToDifferentProperty(s, "Admin Privileges")
			}
		}
		if s != "" {
			conflict := tcc.newConflict(sourceProject.DisplayName, targetProject.DisplayName, sourceProject.ProjectKey, targetProject.ProjectKey, s)
			conflicts = append(conflicts, conflict)
			return conflicts, true
		}

	}
	return conflicts, result
}

func (tcc *TransferConfigCommand) checkIfSameAdminPrivilige(source, target *accessServices.AdminPrivileges) bool {
	if source == nil && target == nil {
		return true
	}

	if source == nil || target == nil {
		return false
	}

	// if source and target Admin priviliges are not nil then we have to check all pointer admin privilige have
	manageMember := tcc.checkIfsameBoolPointer(source.ManageMembers, target.ManageMembers)
	manageResouce := tcc.checkIfsameBoolPointer(source.ManageResources, target.ManageResources)
	indexResouce := tcc.checkIfsameBoolPointer(source.IndexResources, target.IndexResources)
	return (manageMember && manageResouce && indexResouce)

}

func (tcc *TransferConfigCommand) checkIfsameBoolPointer(source, target *bool) bool {
	if source != nil && target != nil {
		if *source != *target {
			return false
		}
	}
	if source == nil || target == nil {
		if source != nil || target != nil {
			return false
		}
	}
	return true
}

func (tcc *TransferConfigCommand) addToDifferentProperty(s, property string) string {
	if s == "" {
		s = property
		return s
	}
	s = s + ", " + property
	return s
}

func (tcc *TransferConfigCommand) tryPing(targetServicesManager artifactory.ArtifactoryServicesManager) error {
	_, err := targetServicesManager.Ping()
	return err
}

func (tcc *TransferConfigCommand) validateMergeArtifactoryServers(targetServicesManager artifactory.ArtifactoryServicesManager, sourceArtifactoryVersion string, minRequiredVersion string) (bool, error) {
	// if version is less than 7.0.0 projects are not defined, and we don't have to transfer them
	transferProjects := true
	if !version.NewVersion(sourceArtifactoryVersion).AtLeast(minRequiredVersion) {
		transferProjects = false
	}

	// Avoid exporting and importing to the same server
	log.Info("Verifying source and target servers are different...")
	if tcc.sourceServerDetails.GetArtifactoryUrl() == tcc.targetServerDetails.GetArtifactoryUrl() {
		return false, errorutils.CheckErrorf("The source and target Artifactory servers are identical, but should be different.")
	}

	// check correctness of Authorization
	log.Info("Checking validation of your authorization methods..")
	if tcc.tryPing(targetServicesManager) != nil {
		return false, errorutils.CheckErrorf("The target's access token is not correct, please provide an availble access token.")
	}
	return transferProjects, nil
}

// Create a csv summary of all conflicts
func (tcc *TransferConfigCommand) createConflictsCSVSummary(conflicts []ProjectConflict, timeStarted time.Time) (csvPath string, err error) {
	// Create CSV file
	summaryCsv, err := loguitils.CreateCustomLogFile(fmt.Sprintf("transfer-config-conflicts-%s.csv", timeStarted.Format(loguitils.DefaultLogTimeLayout)))
	if err != nil {
		return
	}
	csvPath = summaryCsv.Name()
	defer func() {
		e := summaryCsv.Close()
		if err == nil {
			err = e
		}
	}()
	// Marshal JSON typed FileWithLongProperty array to CSV file
	err = errorutils.CheckError(gocsv.MarshalFile(conflicts, summaryCsv))
	return
}

func (tcc *TransferConfigCommand) mergeRepositories(sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) (err error) {
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
	var repoConflicts []RepositoryConflict
	for _, sourceRepo := range *sourceRepos {
		// Check if repository is filtered
		var shouldIncludeRepo bool
		shouldIncludeRepo, err = tcc.getRepoFilter().ShouldIncludeRepository(sourceRepo.Key)
		if err != nil {
			return
		}
		if !shouldIncludeRepo {
			continue
		}

		if targetRepo, exists := targetReposMap[sourceRepo.Key]; exists {
			// Repository exists on target, need to compare

			var reposDiff []string
			reposDiff, err = compareRepositories(sourceRepo, targetRepo, sourceServiceManager, targetServiceManager)
			if err != nil {
				return
			}
			if len(reposDiff) != 0 {
				// Conflicts found, adding to conflicts CSV
				diffProperty := strings.Join(reposDiff, ",")
				conflict := RepositoryConflict{sourceRepo.Key, diffProperty}
				repoConflicts = append(repoConflicts, conflict)
			}
		} else {
			reposToTransfer = append(reposToTransfer, sourceRepo.Key)
		}
	}
	if len(reposToTransfer) > 0 {
		err = transferRepositoriesToTarget(reposToTransfer, sourceServiceManager, targetServiceManager)
	}
	path, err := tcc.createRepositoryConflictsCSVSummary(repoConflicts, time.Now())
	log.Info(fmt.Sprintf("Founded %d Repository Conflicts projects between the source service and the target, check in csv file we created for you in %s", len(repoConflicts), path))

	return
}

func compareRepositories(sourceRepoBaseDetails, targetRepoBaseDetails services.RepositoryDetails, sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) (diff []string, err error) {
	// Compare basic repository details
	diff, err = compareInterfaces(sourceRepoBaseDetails, targetRepoBaseDetails, "Url")
	if err != nil && len(diff) != 0 {
		return
	}

	// Base details is equal, compare full repository details
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
	diff, err = compareInterfaces(sourceRepoFullDetails, targetRepoFullDetails)
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

// Create a csv summary of all conflicts
func (tcc *TransferConfigCommand) createRepositoryConflictsCSVSummary(conflicts []RepositoryConflict, timeStarted time.Time) (csvPath string, err error) {
	// Create CSV file
	summaryCsv, err := loguitils.CreateCustomLogFile(fmt.Sprintf("transfer-config-repository-conflicts-%s.csv", timeStarted.Format(loguitils.DefaultLogTimeLayout)))
	if err != nil {
		return
	}
	csvPath = summaryCsv.Name()
	defer func() {
		e := summaryCsv.Close()
		if err == nil {
			err = e
		}
	}()
	// Marshal JSON typed FileWithLongProperty array to CSV file
	err = errorutils.CheckError(gocsv.MarshalFile(conflicts, summaryCsv))
	return
}
