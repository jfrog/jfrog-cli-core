package terraform

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreparePrerequisites(t *testing.T) {
	terraformPublish := NewTerraformPublishCommandArgs()
	terraformArgs := []string{"--namespace=name", "--provider=aws", "--tag=v0.1.2", "--exclusions=*test*;*ignore*"}
	assert.NoError(t, terraformPublish.extractTerraformPublishOptionsFromArgs(terraformArgs))
	assert.Equal(t, "name", terraformPublish.namespace)
	assert.Equal(t, "aws", terraformPublish.provider)
	assert.Equal(t, "v0.1.2", terraformPublish.tag)
	assert.Equal(t, []string{"*test*", "*ignore*"}, terraformPublish.exclusions)
	// Add unknown flag
	terraformArgs = []string{"--namespace=name", "--provider=aws", "--tag=v0.1.2", "--exclusions=*test*;*ignore*", "--unknown-flag=value"}
	assert.EqualError(t, terraformPublish.extractTerraformPublishOptionsFromArgs(terraformArgs), "Unknown flag:--unknown-flag. for a terraform publish command please provide --namespace, --provider, --tag and optionally --exclusions.")
}

func TestCheckIfTerraformModule(t *testing.T) {
	dirPath := filepath.Join("..", "testdata", "terraform")
	// Check terraform module directory which contain files with a ".tf" extension.
	isModule, err := checkIfTerraformModule(dirPath)
	assert.NoError(t, err)
	assert.True(t, isModule)
	// Check npm directory which doesn't contain files with a ".tf" extension.
	dirPath = filepath.Join("..", "testdata", "npm")
	isModule, err = checkIfTerraformModule(dirPath)
	assert.NoError(t, err)
	assert.False(t, isModule)
}
