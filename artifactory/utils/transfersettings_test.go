package utils

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestNoConfig(t *testing.T) {
	// Set testing environment
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Load transfer settings and make sure nil is returned
	settings, err := LoadTransferSettings()
	assert.NoError(t, err)
	assert.Nil(t, settings)
}

func TestSaveAndLoad(t *testing.T) {
	// Set testing environment
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Save transfer settings with 10 threads
	conf := &TransferSettings{ThreadsNumber: 10}
	assert.NoError(t, SaveTransferSettings(conf))

	// Load transfer settings and make sure the number of threads is 10
	settings, err := LoadTransferSettings()
	assert.NoError(t, err)
	assert.Equal(t, 10, settings.ThreadsNumber)
}
