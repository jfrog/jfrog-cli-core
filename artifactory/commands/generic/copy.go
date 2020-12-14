package generic

import (
	"errors"

	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type CopyCommand struct {
	GenericCommand
	threads int
}

func NewCopyCommand() *CopyCommand {
	return &CopyCommand{GenericCommand: *NewGenericCommand()}
}

func (cc *CopyCommand) Threads() int {
	return cc.threads
}

func (cc *CopyCommand) SetThreads(threads int) *CopyCommand {
	cc.threads = threads
	return cc
}

func (cc *CopyCommand) CommandName() string {
	return "rt_copy"
}

// Copies the artifacts using the specified move pattern.
func (cc *CopyCommand) Run() error {
	// Create Service Manager:
	servicesManager, err := utils.CreateServiceManagerWithThreads(cc.rtDetails, cc.dryRun, cc.threads)
	if err != nil {
		return err
	}

	var errorOccurred = false
	var copyParamsArray []services.MoveCopyParams
	// Create CopyParams for all File-Spec groups.
	for i := 0; i < len(cc.spec.Files); i++ {
		copyParams, err := getCopyParams(cc.spec.Get(i))
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		copyParamsArray = append(copyParamsArray, copyParams)
	}

	// Perform copy.
	totalCopied, totalFailed, err := servicesManager.Copy(copyParamsArray...)
	if err != nil {
		errorOccurred = true
		log.Error(err)
	}
	cc.result.SetSuccessCount(totalCopied)
	cc.result.SetFailCount(totalFailed)

	if errorOccurred {
		return errors.New("Copy finished with errors, please review the logs.")
	}
	return err
}

func getCopyParams(f *spec.File) (copyParams services.MoveCopyParams, err error) {
	copyParams = services.NewMoveCopyParams()
	copyParams.ArtifactoryCommonParams = f.ToArtifactoryCommonParams()
	copyParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}
	copyParams.ExcludeArtifacts, err = f.IsExcludeArtifacts(false)
	if err != nil {
		return
	}
	copyParams.IncludeDeps, err = f.IsIncludeDeps(false)
	if err != nil {
		return
	}
	copyParams.Flat, err = f.IsFlat(false)
	if err != nil {
		return
	}
	return
}
