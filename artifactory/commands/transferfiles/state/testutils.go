package state

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
)

func InitStateTest(t *testing.T) (stateManager *TransferStateManager, cleanUp func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)

	stateManager, err = NewTransferStateManager(true)
	assert.NoError(t, err)

	undoSaveInterval := SetAutoSaveState()
	return stateManager, func() {
		undoSaveInterval()
		cleanUpJfrogHome()
	}
}

// Set the state's save-interval to 0 so every action will be persisted and data can be asserted.
func SetAutoSaveState() (cleanUp func()) {
	previousSaveInterval := stateAndStatusSaveIntervalSecs
	stateAndStatusSaveIntervalSecs = 0
	return func() {
		stateAndStatusSaveIntervalSecs = previousSaveInterval
	}
}
