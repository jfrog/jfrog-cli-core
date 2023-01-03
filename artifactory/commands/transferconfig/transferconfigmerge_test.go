package transferconfig

import (
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	QuotaNumber = 1073741825
)

var (
	tcc TransferConfigCommand
)

func init() {
	tcc = TransferConfigCommand{
		sourceServerDetails:  nil,
		targetServerDetails:  nil,
		dryRun:               false,
		force:                false,
		verbose:              false,
		merge:                false,
		includeReposPatterns: nil,
		excludeReposPatterns: nil,
		workingDir:           "",
	}
}

func getTwoProject(sameKey, sameName, sameDescription, sameAdmin, sameQuotaBytes, sameSoftLimit bool) (source, target services.Project) {
	sourceKey := "ProjectKey"
	targetKey := sourceKey
	sourceName := "ProjectName"
	targetName := sourceName
	sourceDescription := "ProjectDescription"
	targetDescription := sourceDescription
	sourceAdmin := &services.AdminPrivileges{nil, nil, nil}
	targetAdmin := &services.AdminPrivileges{nil, nil, nil}
	sourceQuotaBytes := float64(QuotaNumber)
	targetQuotaBytes := float64(QuotaNumber)
	b := true
	if !sameKey {
		targetKey = sourceKey + "Target"
	}
	if !sameName {
		targetName = sourceName + "Target"
	}
	if !sameDescription {
		targetDescription = sourceDescription + "Target"
	}
	if !sameAdmin {
		targetAdmin.ManageMembers = &b

	}
	var sourceSoftLimit *bool = nil
	var targetSoftLimit *bool = nil

	if !sameSoftLimit {
		targetSoftLimit = &b
	}
	if !sameQuotaBytes {
		targetQuotaBytes = targetQuotaBytes + 125
	}
	source = services.Project{sourceName, sourceDescription, sourceAdmin, sourceSoftLimit, sourceQuotaBytes, sourceKey}
	target = services.Project{targetName, targetDescription, targetAdmin, targetSoftLimit, targetQuotaBytes, targetKey}
	return
}

func createAndVlidateConflicts(t *testing.T) []ProjectConflict {
	var conflicts []ProjectConflict
	source, target := getTwoProject(true, false, true, true, true, true)

	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 1, len(conflicts))
	//Checking if we skip transferring project
	source, target = getTwoProject(true, true, true, true, true, true)
	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 1, len(conflicts))
	source, target = getTwoProject(true, true, false, true, true, true)
	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 2, len(conflicts))
	source, target = getTwoProject(true, true, true, true, false, true)
	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 3, len(conflicts))
	source, target = getTwoProject(true, true, true, true, true, false)
	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 4, len(conflicts))
	source, target = getTwoProject(true, true, true, false, false, false)
	conflicts, _ = tcc.findConflict(source, target, conflicts)
	assert.Equal(t, 5, len(conflicts))
	return conflicts
}

func TestCreateConflictCSV(t *testing.T) {
	conflicts := createAndVlidateConflicts(t)
	_, err := tcc.createConflictsCSVSummary(conflicts, time.Now())
	assert.NoError(t, err)
}
