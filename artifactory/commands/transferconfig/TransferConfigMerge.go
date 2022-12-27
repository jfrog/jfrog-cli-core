package transferconfig

import (
	"fmt"
	"github.com/gocarina/gocsv"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	loguitils "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

type ProjectConflict struct {
	SourceProjectName string `json:"source_project_name,omitempty"`
	TargetProjectName string `json:"target_project_name,omitempty"`
	SourceProjectKey  string `json:"source_project_key,omitempty"`
	TargetProjectKey  string `json:"target_project_key,omitempty"`
	DifferentProperty string `json:"different_property,omitempty"`
}

func (tcc *TransferConfigCommand) newConflict(sourceProjectName, targetProjectName, sourceProjectKey, targetProjectKey, differentProperty string) ProjectConflict {
	conflict := ProjectConflict{SourceProjectName: sourceProjectName, TargetProjectName: targetProjectName, SourceProjectKey: sourceProjectKey, TargetProjectKey: targetProjectKey, DifferentProperty: differentProperty}
	return conflict
}

func (tcc *TransferConfigCommand) RunMergeCommand(sourceServiceManager, targetServiceManager artifactory.ArtifactoryServicesManager, sourceArtifactoryVersion string) (err error) {
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 1/3 - Preparations ==========")))

	// todo check access token and admin token if 401
	// todo add the ping to client in merge validation function
	// Make sure that the source and target Artifactory servers are different and that the target Artifactory is empty
	if err = tcc.validateArtifactoryServers(targetServiceManager, sourceArtifactoryVersion, minArtifactoryMergeVersion); err != nil {
		return
	}
	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 2/3 - Getting Projects and Repositories from Source ==========")))
	sourceServiceAccessManager, err := access.New(sourceServiceManager.GetConfig())
	sourceprojects, err := sourceServiceAccessManager.GetAllProjects()
	if err != nil {
		return err
	}

	log.Info(coreutils.PrintTitle(coreutils.PrintBold("========== Phase 3/3 - Putting Projects and Repositories to Target ==========")))
	targetServiceAccessManager, err := access.New(targetServiceManager.GetConfig())
	targerprojects, err := targetServiceAccessManager.GetAllProjects()
	if err != nil {
		return err
	}

	var projectConflicts []ProjectConflict

	var isConflict bool
	for _, sourceProject := range sourceprojects {
		for _, targetProject := range targerprojects {

			projectConflicts, isConflict = tcc.validateProjectConflict(sourceProject, targetProject, projectConflicts)

		}
		if !isConflict {
			err := targetServiceAccessManager.CreateProject(services.ProjectParams{ProjectDetails: sourceProject})
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

	return nil
}

func (tcc *TransferConfigCommand) validateProjectConflict(sourceProject, targetProject services.Project, conflicts []ProjectConflict) ([]ProjectConflict, bool) {
	if sourceProject.ProjectKey == targetProject.ProjectKey || sourceProject.DisplayName == targetProject.DisplayName {
		s := ""
		if sourceProject.DisplayName != targetProject.DisplayName {
			s = tcc.addToDifferentProperty(s, "Display Name")
		}
		if sourceProject.ProjectKey != targetProject.ProjectKey {
			s = tcc.addToDifferentProperty(s, "Project Key")
		}
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

		if s != "" {
			conflict := tcc.newConflict(sourceProject.DisplayName, targetProject.DisplayName, sourceProject.ProjectKey, targetProject.ProjectKey, s)
			conflicts = append(conflicts, conflict)
			return conflicts, true
		}

	}
	return conflicts, false
}

func (tcc *TransferConfigCommand) checkIfSameAdminPrivilige(source, target *services.AdminPrivileges) bool {
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

func (tcc *TransferConfigCommand) tryPing(targetServicesManager artifactory.ArtifactoryServicesManager) bool {

	return true
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
	if !(tcc.tryPing(targetServicesManager)) {
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
