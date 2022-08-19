package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadRunStatus(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = repo4Key
	stateManager.CurrentRepoPhase = 2

	assert.NoError(t, stateManager.saveTransferRunStatus())
	actualStatus, err := loadTransferRunStatus()
	assert.NoError(t, err)
	assert.Equal(t, stateManager.TransferRunStatus, *actualStatus)
}
