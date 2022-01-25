package terraform

import (
	"errors"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientservicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
	dirPath := filepath.Join("..", "testdata", "terraform", "terraformproject")
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
	terraformTestDir := filepath.Join("..", "testdata", "terraform")
	terraformPublish := NewTerraformPublishCommand().SetServerDetails(&config.ServerDetails{})
	uploadSummary := clientservicesutils.NewResult(cliutils.Threads)
	producerConsumer := newTestRunner()
	errorsQueue := clientutils.NewErrorsQueue(1)
	tests := []struct {
		name     string
		path     string
		testFunc AddTaskWithErrorFunc
	}{
		{name: "testEmptyModule", path: filepath.Join(terraformTestDir, "empty"), testFunc: mockEmptyModule},
		{name: "mockAddTaskWithErrorFunc", path: filepath.Join(terraformTestDir, "terraformproject"), testFunc: testTerraformModule},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NoError(t, terraformPublish.walkDirAndUploadTerraformModules(test.path, producerConsumer, errorsQueue, uploadSummary, test.testFunc))
			assert.NoError(t, errorsQueue.GetError())
		})
	}
}

func mockEmptyModule(_ parallel.Runner, _ *services.UploadService, _ *specutils.Result, _ string, _ *services.ArchiveUploadData, _ *clientutils.ErrorsQueue) (int, error) {
	return 0, errors.New("Failed: testing empty directory. this function shouldn't be called. ")
}
func testTerraformModule(_ parallel.Runner, _ *services.UploadService, _ *specutils.Result, _ string, archiveData *services.ArchiveUploadData, _ *clientutils.ErrorsQueue) (int, error) {
	paths, err := mockAddTaskWithErrorFunc(archiveData)
	if err != nil {
		return 0, err
	}
	return 0, tests.ValidateListsIdentical([]string{"main.tf", "test/file.tf"}, paths)
}

func mockAddTaskWithErrorFunc(archiveData *services.ArchiveUploadData) (paths []string, err error) {
	archiveData.GetWriter().GetFilePath()
	archiveDataReader := content.NewContentReader(archiveData.GetWriter().GetFilePath(), archiveData.GetWriter().GetArrayKey())
	defer func() {
		e := archiveDataReader.Close()
		if e == nil {
			err = e
		}
	}()
	for uploadData := new(services.UploadData); archiveDataReader.NextRecord(uploadData) == nil; uploadData = new(services.UploadData) {
		paths = append(paths, uploadData.Artifact.TargetPathInArchive)
	}
	return paths, nil
}

type testRunner struct {
	archivesData *[]*services.ArchiveUploadData
}

func newTestRunner() *testRunner {
	return &testRunner{archivesData: &[]*services.ArchiveUploadData{}}
}

func (tr *testRunner) AddTask(tf parallel.TaskFunc) (int, error) {
	return 0, nil
}

func (tr *testRunner) AddTaskWithError(tf parallel.TaskFunc, ef parallel.OnErrorFunc) (int, error) {
	return 0, nil
}

func (tr *testRunner) Run() {

}

func (tr *testRunner) Done() {

}

func (tr *testRunner) Cancel() {

}

func (tr *testRunner) Errors() map[int]error {
	return nil
}
