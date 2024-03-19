package generic

import (
	ioutils "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
)

type DeleteCommand struct {
	GenericCommand
	threads int
}

func NewDeleteCommand() *DeleteCommand {
	return &DeleteCommand{GenericCommand: *NewGenericCommand()}
}

func (dc *DeleteCommand) Threads() int {
	return dc.threads
}

func (dc *DeleteCommand) SetThreads(threads int) *DeleteCommand {
	dc.threads = threads
	return dc
}

func (dc *DeleteCommand) CommandName() string {
	return "rt_delete"
}

func (dc *DeleteCommand) Run() (err error) {
	reader, err := dc.GetPathsToDelete()
	if err != nil {
		return
	}
	defer ioutils.Close(reader, &err)
	allowDelete := true
	if !dc.quiet {
		allowDelete, err = utils.ConfirmDelete(reader)
		if err != nil {
			return
		}
	}
	if allowDelete {
		var successCount int
		var failedCount int
		successCount, failedCount, err = dc.DeleteFiles(reader)
		result := dc.Result()
		result.SetFailCount(failedCount)
		result.SetSuccessCount(successCount)
	}
	return
}

func (dc *DeleteCommand) GetPathsToDelete() (contentReader *content.ContentReader, err error) {
	serverDetails, err := dc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return
	}
	servicesManager, err := utils.CreateServiceManager(serverDetails, dc.retries, dc.retryWaitTimeMilliSecs, dc.DryRun())
	if err != nil {
		return
	}
	var temp []*content.ContentReader
	defer func() {
		for _, reader := range temp {
			ioutils.Close(reader, &err)
		}
	}()
	for i := 0; i < len(dc.Spec().Files); i++ {
		var deleteParams services.DeleteParams
		deleteParams, err = getDeleteParams(dc.Spec().Get(i))
		if err != nil {
			return
		}
		var reader *content.ContentReader
		reader, err = servicesManager.GetPathsToDelete(deleteParams)
		if err != nil {
			return
		}
		temp = append(temp, reader)
	}
	tempMergedReader, err := content.MergeReaders(temp, content.DefaultKey)
	if err != nil {
		return nil, err
	}
	defer ioutils.Close(tempMergedReader, &err)
	// After merge, remove top chain dirs as we may encounter duplicates and collisions between files and directories to delete.
	// For example:
	// Reader1: {"a"}
	// Reader2: {"a/b","a/c"}
	// After merge, received a Reader: {"a","a/b","a/c"}.
	// If "a" is deleted prior to "a/b" or "a/c", the delete operation returns a failure.
	contentReader, err = clientutils.ReduceTopChainDirResult(clientutils.ResultItem{}, tempMergedReader)
	return
}

func (dc *DeleteCommand) DeleteFiles(reader *content.ContentReader) (successCount, failedCount int, err error) {
	serverDetails, err := dc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return 0, 0, err
	}
	servicesManager, err := utils.CreateDeleteServiceManager(serverDetails, dc.Threads(), dc.retries, dc.retryWaitTimeMilliSecs, dc.DryRun())
	if err != nil {
		return 0, 0, err
	}
	deletedCount, err := servicesManager.DeleteFiles(reader)
	if err != nil {
		return 0, 0, err
	}
	length, err := reader.Length()
	if err != nil {
		return 0, 0, err
	}
	return deletedCount, length - deletedCount, err
}

func getDeleteParams(f *spec.File) (deleteParams services.DeleteParams, err error) {
	deleteParams = services.NewDeleteParams()
	deleteParams.CommonParams, err = f.ToCommonParams()
	if err != nil {
		return
	}
	deleteParams.ExcludeArtifacts, err = f.IsExcludeArtifacts(false)
	if err != nil {
		return
	}
	deleteParams.IncludeDeps, err = f.IsIncludeDeps(false)
	if err != nil {
		return
	}
	deleteParams.Recursive, err = f.IsRecursive(true)
	return
}
