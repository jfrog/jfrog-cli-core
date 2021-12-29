package generic

import (
	"errors"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type MoveCommand struct {
	GenericCommand
	threads int
}

func NewMoveCommand() *MoveCommand {
	return &MoveCommand{GenericCommand: *NewGenericCommand()}
}

func (mc *MoveCommand) Threads() int {
	return mc.threads
}

func (mc *MoveCommand) SetThreads(threads int) *MoveCommand {
	mc.threads = threads
	return mc
}

// Moves the artifacts using the specified move pattern.
func (mc *MoveCommand) Run() error {
	// Create Service Manager:
	servicesManager, err := utils.CreateServiceManagerWithThreads(mc.serverDetails, mc.DryRun(), mc.threads, mc.retries, mc.retryWaitTimeMilliSecs)
	if err != nil {
		return err
	}

	var errorOccurred = false
	var moveParamsArray []services.MoveCopyParams
	// Create MoveParams for all File-Spec groups.
	for i := 0; i < len(mc.Spec().Files); i++ {
		moveParams, err := getMoveParams(mc.Spec().Get(i))
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		moveParamsArray = append(moveParamsArray, moveParams)
	}

	// Perform move.
	totalMoved, totalFailed, err := servicesManager.Move(moveParamsArray...)
	if err != nil {
		errorOccurred = true
		log.Error(err)
	}
	mc.result.SetSuccessCount(totalMoved)
	mc.result.SetFailCount(totalFailed)

	if errorOccurred {
		return errors.New("Move finished with errors, please review the logs.")
	}
	return err
}

func (mc *MoveCommand) CommandName() string {
	return "rt_move"
}

func getMoveParams(f *spec.File) (moveParams services.MoveCopyParams, err error) {
	moveParams = services.NewMoveCopyParams()
	moveParams.CommonParams, err = f.ToCommonParams()
	if err != nil {
		return
	}
	moveParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}
	moveParams.ExcludeArtifacts, err = f.IsExcludeArtifacts(false)
	if err != nil {
		return
	}
	moveParams.IncludeDeps, err = f.IsIncludeDeps(false)
	if err != nil {
		return
	}
	moveParams.Flat, err = f.IsFlat(false)
	if err != nil {
		return
	}
	return
}
