package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadRunStatus(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = newRepositoryTransferState(repo4Key).CurrentRepo
	stateManager.CurrentRepoPhase = 2

	assert.NoError(t, stateManager.persistTransferRunStatus())
	actualStatus, exists, err := loadTransferRunStatus()
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, transferRunStatusVersion, actualStatus.Version)
	actualStatus.TimeEstimationManager.stateManager = stateManager
	assert.Equal(t, stateManager.TransferRunStatus, actualStatus)
}
