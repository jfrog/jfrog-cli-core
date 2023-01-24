package terraform

import (
	"errors"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientServicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestPreparePrerequisites(t *testing.T) {
	terraformPublishArgs := NewTerraformPublishCommandArgs()
	terraformArgs := []string{"--namespace=name", "--provider=aws", "--tag=v0.1.2", "--exclusions=*test*;*ignore*"}
	assert.NoError(t, terraformPublishArgs.extractTerraformPublishOptionsFromArgs(terraformArgs))
	assert.Equal(t, "name", terraformPublishArgs.namespace)
	assert.Equal(t, "aws", terraformPublishArgs.provider)
	assert.Equal(t, "v0.1.2", terraformPublishArgs.tag)
	assert.Equal(t, []string{"*test*", "*ignore*"}, terraformPublishArgs.exclusions)
	// Add unknown flag
	terraformArgs = []string{"--namespace=name", "--provider=aws", "--tag=v0.1.2", "--exclusions=*test*;*ignore*", "--unknown-flag=value"}
	assert.EqualError(t, terraformPublishArgs.extractTerraformPublishOptionsFromArgs(terraformArgs), "Unknown flag:--unknown-flag. for a terraform publish command please provide --namespace, --provider, --tag and optionally --exclusions.")
}

func TestCheckIfTerraformModule(t *testing.T) {
	dirPath := filepath.Join("..", "testdata", "terraform", "terraform_project")
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

func TestWalkDirAndUploadTerraformModules(t *testing.T) {
	t.Run("testEmptyModule", func(t *testing.T) { runTerraformTest(t, "empty", mockEmptyModule) })
	t.Run("mockProduceTaskFunc", func(t *testing.T) {
		runTerraformTest(t, "terraform_project", getTaskMock(t, []string{"a.tf", "test_dir/b.tf", "test_dir/submodules/test_submodule/c.tf"}))
	})
	t.Run("testSpecialChar", func(t *testing.T) {
		runTerraformTest(t, "terra$+~&^a#", getTaskMock(t, []string{"a.tf", "$+~&^a#@.tf"}))
	})
	t.Run("testExcludeTestDirectory", func(t *testing.T) {
		runTerraformTestWithExclusions(t, "terraform_project", getTaskMock(t, []string{"a.tf"}), []string{"*test_dir*", "*@*"})
	})
	t.Run("testExcludeTestSubmoduleModule", func(t *testing.T) {
		runTerraformTestWithExclusions(t, filepath.Join("terraform_project", "test_dir"), getTaskMock(t, []string{"b.tf"}), []string{"*test_sub*", "*@*"})
	})
	t.Run("testExcludeSpecialChar", func(t *testing.T) {
		runTerraformTestWithExclusions(t, "terra$+~&^a#", getTaskMock(t, []string{"a.tf"}), []string{"*test_dir*", "*@*"})
	})
}

func getTerraformTestDir(path string) string {
	return filepath.Join("..", "testdata", "terraform", path)
}

func runTerraformTest(t *testing.T, subDir string, testFunc ProduceTaskFunc) {
	runTerraformTestWithExclusions(t, subDir, testFunc, []string{})
}

func runTerraformTestWithExclusions(t *testing.T, subDir string, testFunc ProduceTaskFunc, exclusions []string) {
	terraformPublish := NewTerraformPublishCommand()
	terraformPublish.setServerDetails(&config.ServerDetails{})
	terraformPublish.exclusions = exclusions
	uploadSummary := getNewUploadSummaryMultiArray()
	producerConsumer := parallel.NewRunner(threads, 20000, false)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	assert.NoError(t, terraformPublish.walkDirAndUploadTerraformModules(getTerraformTestDir(subDir), producerConsumer, errorsQueue, uploadSummary, testFunc))
	assert.NoError(t, errorsQueue.GetError())
}

// In order to verify the walk function has visited the correct dirs, and that the correct files were collected/excluded, we run a dry run upload on the files without an archive.
func getTaskMock(t *testing.T, expectedPaths []string) func(parallel.Runner, *config.ServerDetails, *[][]*clientServicesUtils.OperationSummary, *services.UploadParams, *clientUtils.ErrorsQueue) (int, error) {
	return func(_ parallel.Runner, serverDetails *config.ServerDetails, _ *[][]*clientServicesUtils.OperationSummary, uploadParams *services.UploadParams, _ *clientUtils.ErrorsQueue) (int, error) {
		uploadParams.Target = uploadParams.TargetPathInArchive
		uploadParams.TargetPathInArchive = ""
		uploadParams.Archive = ""
		uploadParams.Target = "dummy-repo/{1}"

		summary, err := createServiceManagerAndUpload(serverDetails, uploadParams, true)
		assert.NoError(t, err)
		if !assert.NotNil(t, summary) {
			return 0, nil
		}
		artifacts, err := readArtifactsFromSummary(summary)
		assert.NoError(t, err)
		var actualPaths []string
		for _, artifact := range artifacts {
			actualPaths = append(actualPaths, artifact.Path)
		}
		assert.ElementsMatch(t, actualPaths, expectedPaths)
		return 0, nil
	}
}

func mockEmptyModule(_ parallel.Runner, _ *config.ServerDetails, _ *[][]*clientServicesUtils.OperationSummary, _ *services.UploadParams, _ *clientUtils.ErrorsQueue) (int, error) {
	return 0, errors.New("failed: testing empty directory. this function shouldn't be called. ")
}
