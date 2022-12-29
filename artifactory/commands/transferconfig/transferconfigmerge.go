package transferconfig

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"golang.org/x/exp/slices"
	"reflect"
)

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
				// Todo: Add to conflicts CSV
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
		err = targetServiceManager.CreateRepositoryWithJsonParams(params, repoKey)
		if err != nil {
			return
		}
	}
	return nil
}
