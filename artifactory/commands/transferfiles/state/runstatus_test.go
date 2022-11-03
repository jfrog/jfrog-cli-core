package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadRunStatus(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = repo4Key
	stateManager.CurrentRepoPhase = 2

	assert.NoError(t, stateManager.persistTransferRunStatus())
	actualStatus, err := loadTransferRunStatus()
	assert.NoError(t, err)
	actualStatus.TimeEstimationManager.stateManager = stateManager
	assert.Equal(t, stateManager.TransferRunStatus, *actualStatus)
}
