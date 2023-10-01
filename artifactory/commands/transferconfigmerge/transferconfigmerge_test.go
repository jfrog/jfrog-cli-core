package transferconfigmerge

import (
	"github.com/jfrog/jfrog-client-go/access/services"
	artifactoryServices "github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

const (
	quotaNumber = 1073741825
)

func TestCreateAndValidateConflicts(t *testing.T) {
	tests := []struct {
		sameKey           bool
		sameName          bool
		sameDescription   bool
		sameAdmin         bool
		sameQuotaBytes    bool
		sameSoftLimit     bool
		expectedDiffCount int
	}{
		{true, true, true, true, true, true, 0},
		{true, true, true, true, true, false, 1},
		{true, true, true, true, false, false, 2},
		{true, true, true, false, false, false, 3},
		{true, true, false, false, false, false, 4},
		{true, false, false, false, false, false, 5},
		{false, false, false, false, false, false, 6},
	}
	for _, test := range tests {
		source, target := createProjects(test.sameKey, test.sameName, test.sameDescription, test.sameAdmin, test.sameQuotaBytes, test.sameSoftLimit)
		conflicts, err := compareProjects(source, target)
		assert.NoError(t, err)
		diffCount := 0
		if conflicts != nil {
			diffCount = len(strings.Split(conflicts.DifferentProperties, ";"))
		}
		assert.Equal(t, test.expectedDiffCount, diffCount)
	}
}

func createProjects(sameKey, sameName, sameDescription, sameAdmin, sameQuotaBytes, sameSoftLimit bool) (source, target services.Project) {
	sourceKey := "ProjectKey"
	targetKey := sourceKey
	sourceName := "ProjectName"
	targetName := sourceName
	sourceDescription := "ProjectDescription"
	targetDescription := sourceDescription
	sourceAdmin := &services.AdminPrivileges{}
	targetAdmin := &services.AdminPrivileges{}
	sourceQuotaBytes := float64(quotaNumber)
	targetQuotaBytes := float64(quotaNumber)
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
		targetAdmin.ManageMembers = clientutils.Pointer(true)
		targetAdmin.IndexResources = clientutils.Pointer(true)
	}
	var sourceSoftLimit = clientutils.Pointer(false)
	var targetSoftLimit = clientutils.Pointer(false)

	if !sameSoftLimit {
		targetSoftLimit = clientutils.Pointer(true)
	}
	if !sameQuotaBytes {
		targetQuotaBytes += 125
	}
	source = services.Project{DisplayName: sourceName, Description: sourceDescription, AdminPrivileges: sourceAdmin, SoftLimit: sourceSoftLimit, StorageQuotaBytes: sourceQuotaBytes, ProjectKey: sourceKey}
	target = services.Project{DisplayName: targetName, Description: targetDescription, AdminPrivileges: targetAdmin, SoftLimit: targetSoftLimit, StorageQuotaBytes: targetQuotaBytes, ProjectKey: targetKey}
	return
}

func TestCompareInterfaces(t *testing.T) {
	first := artifactoryServices.DockerRemoteRepositoryParams{}
	first.RemoteRepositoryBaseParams = artifactoryServices.RemoteRepositoryBaseParams{Password: "ppppp"}
	first.Key = "string1"
	first.BlackedOut = clientutils.Pointer(true)
	first.AssumedOfflinePeriodSecs = clientutils.Pointer(1111)
	first.Environments = []string{"111", "aaa"}
	first.ContentSynchronisation = &artifactoryServices.ContentSynchronisation{Enabled: clientutils.Pointer(true)}

	second := artifactoryServices.DockerRemoteRepositoryParams{}
	second.RemoteRepositoryBaseParams = artifactoryServices.RemoteRepositoryBaseParams{Password: "sssss"}
	second.Key = "string2"
	second.BlackedOut = clientutils.Pointer(false)
	second.AssumedOfflinePeriodSecs = clientutils.Pointer(2222)
	second.Environments = []string{"222", "bbb"}
	second.ContentSynchronisation = &artifactoryServices.ContentSynchronisation{Enabled: clientutils.Pointer(false)}

	diff, err := compareInterfaces(first, second, filteredRepoKeys...)
	assert.NoError(t, err)
	// Expect 5 differences (password should be filtered)
	assert.Len(t, strings.Split(diff, ";"), 5)
}
