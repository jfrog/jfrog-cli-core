package transferconfig

import (
	"fmt"
	"github.com/gocarina/gocsv"
	loguitils "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/artifactory"
	services2 "github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

const (
	minArtifactoryMergeVersion = "7.0.0"
)

type RepositoryConflict struct {
	SourceRepoKey     string `json:"source_repo_key,omitempty"`
	TargetRepoKey     string `json:"target_repo_key,omitempty"`
	DifferentProperty string `json:"different_property,omitempty"`
}

func (tcc *TransferConfigCommand) TransferRepositories(sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager) error {
	sourceRepositories, err := sourceServiceManager.GetAllRepositories()
	if err != nil {
		return err
	}
	targetRepositories, err := targetServiceManager.GetAllRepositories()
	if err != nil {
		return err
	}
	var remoteRepos []services2.RemoteRepositoryBaseParams
	var virtualRepos []services2.VirtualRepositoryBaseParams
	var localRepos []services2.LocalRepositoryBaseParams
	var federateRepos []services2.FederatedRepositoryBaseParams
	var repositoryConflicts []RepositoryConflict
	isConflict := false

	for _, sourceRepo := range *sourceRepositories {
		for _, targetRepo := range *targetRepositories {
			// todo check if there is a conflict if not, put it in suitable list
			repositoryConflicts, tempConflict := tcc.validateConflictsAndSortRepositories(sourceRepo, targetRepo, repositoryConflicts)
			isConflict = isConflict || tempConflict
		}
		if !isConflict {

		}
	}
	csvPath, err := tcc.createRepoConflictsCSVSummary(repositoryConflicts, time.Now())
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Founded %d Repository Conflicts projects between the source service and the target, check in csv file we created for you in %s", len(repositoryConflicts), csvPath))
	return nil
}

func (tcc *TransferConfigCommand) sortRepository(sourceServiceManager artifactory.ArtifactoryServicesManager, sourceRepo services2.RepositoryDetails, remote []services2.RemoteRepositoryBaseParams, virtual []services2.VirtualRepositoryBaseParams, local []services2.LocalRepositoryBaseParams, federate []services2.FederatedRepositoryBaseParams) ([]services2.RemoteRepositoryBaseParams, []services2.VirtualRepositoryBaseParams, []services2.LocalRepositoryBaseParams, []services2.FederatedRepositoryBaseParams, error) {
	var err error
	switch sourceRepo.Type {
	case "Virtual":
		err = sourceServiceManager.GetRepository(sourceRepo.Key, services2.VirtualRepositoryBaseParams{})
	case "Remote":
		err = sourceServiceManager.GetRepository(sourceRepo.Key, services2.RemoteRepositoryBaseParams{})
	case "Local":
		err = sourceServiceManager.GetRepository(sourceRepo.Key, services2.LocalRepositoryBaseParams{})
	case "Federate":
		err = sourceServiceManager.GetRepository(sourceRepo.Key, services2.FederatedRepositoryBaseParams{})
	}
	return remote, virtual, local, federate, err
}

func (tcc *TransferConfigCommand) validateConflictsAndSortRepositories(sourceRepo, targetRepo services2.RepositoryDetails, conflicts []RepositoryConflict) ([]RepositoryConflict, bool) {
	s := ""
	if sourceRepo.Key == targetRepo.Key {
		if sourceRepo.Description != targetRepo.Description {
			s = tcc.addToDifferentProperty(s, "Description")
		}
		if sourceRepo.Type != targetRepo.Type {
			s = tcc.addToDifferentProperty(s, "Repo Type")
		}
		if sourceRepo.Rclass != targetRepo.Rclass {
			s = tcc.addToDifferentProperty(s, "Rclass")
		}
		if sourceRepo.PackageType != targetRepo.PackageType {
			s = tcc.addToDifferentProperty(s, "Package type")
		}
		if s != "" {
			conflict := RepositoryConflict{SourceRepoKey: sourceRepo.Key, TargetRepoKey: targetRepo.Key, DifferentProperty: s}
			conflicts = append(conflicts, conflict)
			return conflicts, true
		}
	}
	return conflicts, false
}

// Create a csv summary of all conflicts
func (tcc *TransferConfigCommand) createRepoConflictsCSVSummary(conflicts []RepositoryConflict, timeStarted time.Time) (csvPath string, err error) {
	// Create CSV file
	summaryCsv, err := loguitils.CreateCustomLogFile(fmt.Sprintf("transfer-repo-config-conflicts-%s.csv", timeStarted.Format(loguitils.DefaultLogTimeLayout)))
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
