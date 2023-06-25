package progressbar

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"sync"
	"time"
)

const (
	phase1HeadLine = "Phase 1: Transferring all files in the repository"
	phase2HeadLine = "Phase 2: Transferring newly created and modified files"
)

type transferLabels struct {
	Repositories            string
	Files                   string
	Storage                 string
	Note                    string
	RetryFailureContentNote string
	TransferSpeed           string
	EstimatedTime           string
	TransferFailures        string
	WorkingThreads          string
	RunningFor              string
	DiffStorage             string
	DiffFiles               string
	FailedStorage           string
	FailedFiles             string
}

func formatString(emoji, key string, windows bool) string {
	if len(emoji) > 0 {
		if windows {
			emoji = "●"
		}
		key = emoji + " " + key
	}
	return key
}

func initSProgressBarLabels(windows bool) transferLabels {
	pbs := transferLabels{}
	pbs.RetryFailureContentNote = "In Phase 3 and in subsequent executions, we'll retry transferring the failed files."
	pbs.Repositories = formatString("📦", " Repositories", windows)
	pbs.Files = formatString("📄", " Files", windows)
	pbs.Storage = formatString("🗄 ", " Storage", windows)
	pbs.Note = formatString(" 🟠", " Note: ", windows)
	pbs.TransferSpeed = formatString(" ⚡", " Transfer speed: ", windows)
	pbs.EstimatedTime = formatString(" ⌛", " Estimated time remaining: ", windows)
	pbs.TransferFailures = formatString(" ❌", " Transfer failures: ", windows)
	pbs.WorkingThreads = formatString(" 🧵", " Working threads: ", windows)
	pbs.RunningFor = formatString(" 🏃🏼", " Running for: ", windows)
	pbs.DiffStorage = formatString("🗄 ", " Diff Storage", windows)
	pbs.DiffFiles = formatString("📄", " Diff Files", windows)
	pbs.FailedFiles = formatString("📄", " Failed Files", windows)
	pbs.FailedStorage = formatString("🗄 ", " Failed Storage", windows)
	return pbs
}

type TransferProgressMng struct {
	barMng                *ProgressBarMng
	stateMng              *state.TransferStateManager
	transferLabels        transferLabels
	wg                    sync.WaitGroup
	reposWg               sync.WaitGroup
	allRepos              []string
	ignoreState           bool
	currentRepoShouldStop bool
	generalShouldStop     bool
}

func InitTransferProgressBarMng(state *state.TransferStateManager, allRepos []string) (mng *TransferProgressMng, shouldInit bool, err error) {
	mng = &TransferProgressMng{}
	mng.barMng, shouldInit, err = NewBarsMng()
	mng.stateMng = state
	mng.allRepos = allRepos
	mng.generalShouldStop = false
	mng.transferLabels = initSProgressBarLabels(coreutils.IsWindows())
	return
}

func (tpm *TransferProgressMng) StopCurrentRepoProgressBars(shouldStop bool) {
	tpm.currentRepoShouldStop = shouldStop
}

func (tpm *TransferProgressMng) StopGlobalProgressBars() {
	tpm.generalShouldStop = true
}

func (tpm *TransferProgressMng) NewPhase1ProgressBar() *TasksWithHeadlineProg {
	getVals := func() (transferredStorage, totalStorage, transferresFiles, totalFiles *int64, err error) {
		err = tpm.stateMng.Action(func(state *state.TransferState) error {
			transferredStorage = &tpm.stateMng.CurrentRepo.Phase1Info.TransferredSizeBytes
			totalStorage = &tpm.stateMng.CurrentRepo.Phase1Info.TotalSizeBytes
			transferresFiles = &tpm.stateMng.CurrentRepo.Phase1Info.TransferredUnits
			totalFiles = &tpm.stateMng.CurrentRepo.Phase1Info.TotalUnits
			return nil
		})
		return transferredStorage, totalStorage, transferresFiles, totalFiles, err
	}
	pb := tpm.barMng.newDoubleHeadLineProgressBar(phase1HeadLine, tpm.transferLabels.Storage, tpm.transferLabels.Files, getVals)

	tpm.wg.Add(1)
	go func() {
		defer tpm.wg.Done()
		for {
			if tpm.currentRepoShouldStop {
				return
			}
			ptr1, ptr2, _, _, err := getVals()
			if err != nil {
				log.Error("Error: Couldn't get needed information about transfer status from state")
			}
			if pb == nil {
				log.Error("Error: We Couldn't initialize the progress bar so we can't set values to it")
				return
			}
			if pb.GetTasksProgressBar() != nil {
				pb.GetTasksProgressBar().SetGeneralProgressTotal(*ptr2)
				pb.GetTasksProgressBar().GetBar().SetCurrent(*ptr1)
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return pb
}

func (tpm *TransferProgressMng) NewPhase2ProgressBar() *TasksWithHeadlineProg {
	getVals := func() (transferresStorage, totalStorage, transferredFiles, totalFiles *int64, err error) {
		err = tpm.stateMng.Action(func(state *state.TransferState) error {
			transferresStorage = &tpm.stateMng.CurrentRepo.Phase2Info.TransferredSizeBytes
			totalStorage = &tpm.stateMng.CurrentRepo.Phase2Info.TotalSizeBytes
			transferredFiles = &tpm.stateMng.CurrentRepo.Phase2Info.TransferredUnits
			totalFiles = &tpm.stateMng.CurrentRepo.Phase2Info.TotalUnits
			return nil
		})
		return transferresStorage, totalStorage, transferredFiles, totalFiles, err
	}
	pb := tpm.barMng.newDoubleHeadLineProgressBar(phase2HeadLine, tpm.transferLabels.Storage, tpm.transferLabels.Files, getVals)

	tpm.wg.Add(1)
	go func() {
		defer tpm.wg.Done()
		for {
			if tpm.currentRepoShouldStop {
				return
			}
			ptr1, ptr2, _, _, err := getVals()
			if err != nil {
				log.Error("Error: Couldn't get needed information about transfer status from state")
			}
			if pb == nil {
				log.Error("Error: We Couldn't initialize the progress bar so we can't set values to it")
				return
			}
			if pb.GetTasksProgressBar() != nil {
				pb.GetTasksProgressBar().SetGeneralProgressTotal(*ptr2)
				pb.GetTasksProgressBar().GetBar().SetCurrent(*ptr1)
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return pb
}

func (tpm *TransferProgressMng) NewPhase3ProgressBar() *TasksWithHeadlineProg {
	getVals := func() (transferredStorage, totalStorage, transferredFiles, totalFiles *int64, err error) {
		err = tpm.stateMng.Action(func(state *state.TransferState) error {
			transferredStorage = &tpm.stateMng.CurrentRepo.Phase3Info.TransferredSizeBytes
			totalStorage = &tpm.stateMng.CurrentRepo.Phase3Info.TotalSizeBytes
			transferredFiles = &tpm.stateMng.CurrentRepo.Phase3Info.TransferredUnits
			totalFiles = &tpm.stateMng.CurrentRepo.Phase3Info.TotalUnits
			return nil
		})
		return transferredStorage, totalStorage, transferredFiles, totalFiles, err
	}
	pb := tpm.barMng.newDoubleHeadLineProgressBar(phase2HeadLine, tpm.transferLabels.Storage, tpm.transferLabels.Files, getVals)

	tpm.wg.Add(1)
	go func() {
		defer tpm.wg.Done()
		for {
			if tpm.currentRepoShouldStop {
				return
			}
			ptr1, ptr2, _, _, err := getVals()
			if err != nil {
				log.Error("Error: Couldn't get needed information about transfer status from state")
			}
			if pb == nil {
				log.Error("Error: We Couldn't initialize the progress bar so we can't set values to it")
				return
			}
			if pb.GetTasksProgressBar() != nil {
				pb.GetTasksProgressBar().SetGeneralProgressTotal(*ptr2)
				pb.GetTasksProgressBar().GetBar().SetCurrent(*ptr1)
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return pb
}

func (tpm *TransferProgressMng) NewRepositoriesProgressBar() *TasksWithHeadlineProg {
	getVals := func() (totalRepos, transferredRepos *int64) {
		totalRepos = &tpm.stateMng.TotalRepositories.TotalUnits
		transferredRepos = &tpm.stateMng.TotalRepositories.TransferredUnits
		return transferredRepos, totalRepos
	}
	pb := tpm.barMng.newHeadlineTaskProgressBar(getVals, "Transferring your repositories", tpm.transferLabels.Repositories)

	tpm.reposWg.Add(1)
	go func() {
		defer tpm.reposWg.Done()
		for {
			if tpm.generalShouldStop {
				return
			}
			transferredRepos, totalRepos := getVals()
			if pb == nil {
				log.Error("We Couldn't initialize the progress bar so we can't set values to it")
				return
			}
			if pb.GetTasksProgressBar() != nil {
				pb.GetTasksProgressBar().SetGeneralProgressTotal(*totalRepos)
				pb.GetTasksProgressBar().GetBar().SetCurrent(*transferredRepos)
			}
			time.Sleep(time.Second)
		}
	}()
	return pb
}

func (tpm *TransferProgressMng) NewGeneralProgBar() *TasksProgressBar {
	getVals := func() (transferredStorage, totalStorage, transferredFiles, totalFiles *int64, err error) {
		err = tpm.stateMng.Action(func(state *state.TransferState) error {
			transferredStorage = &tpm.stateMng.TransferRunStatus.OverallTransfer.TransferredSizeBytes
			totalStorage = &tpm.stateMng.OverallTransfer.TotalSizeBytes
			transferredFiles = &tpm.stateMng.OverallTransfer.TransferredUnits
			totalFiles = &tpm.stateMng.OverallTransfer.TotalUnits
			return nil
		})
		return transferredStorage, totalStorage, transferredFiles, totalFiles, err
	}
	pb := tpm.barMng.newDoubleValueProgressBar(getVals, tpm.transferLabels.Storage, tpm.transferLabels.Files)

	tpm.reposWg.Add(1)
	go func() {
		defer tpm.reposWg.Done()
		for {
			if tpm.generalShouldStop {
				return
			}
			transferredStorage, totalStorage, _, _, err := getVals()
			if err != nil {
				log.Error(err)
			}
			pb.SetGeneralProgressTotal(*totalStorage)
			pb.GetBar().SetCurrent(*transferredStorage)
			time.Sleep(time.Second)
		}
	}()
	return pb
}

func (tpm *TransferProgressMng) NewWorkingThreadsProg() *TasksProgressBar {
	getVal := func() (workingThreadsNum int, err error) {
		err = tpm.stateMng.Action(func(state *state.TransferState) error {
			workingThreadsNum = tpm.stateMng.WorkingThreads
			return nil
		})
		return workingThreadsNum, err
	}

	return tpm.barMng.newCounterProgressBar(getVal, tpm.transferLabels.WorkingThreads)
}

func (tpm *TransferProgressMng) GetBarMng() *ProgressBarMng {
	return tpm.barMng
}

func (tpm *TransferProgressMng) NewRunningTimeProgressBar() *TasksProgressBar {
	pb := tpm.barMng.NewStringProgressBar(tpm.transferLabels.RunningFor, func() string {
		runningTime, isRunning, err := state.GetRunningTime()
		if err != nil || !isRunning {
			runningTime = "Running time not available"
		}
		return color.Green.Render(runningTime)
	})
	return pb
}

func (tpm *TransferProgressMng) NewSpeedProgBar() *TasksProgressBar {
	pb := tpm.barMng.NewStringProgressBar(tpm.transferLabels.TransferSpeed, func() string {
		return color.Green.Render(tpm.stateMng.TimeEstimationManager.GetSpeedString())
	})
	return pb
}

func (tpm *TransferProgressMng) NewTimeEstBar() *TasksProgressBar {
	pb := tpm.barMng.NewStringProgressBar(tpm.transferLabels.EstimatedTime, func() string {
		return color.Green.Render(tpm.stateMng.TimeEstimationManager.GetEstimatedRemainingTimeString())
	})
	return pb
}

func (tpm *TransferProgressMng) NewErrorBar() *TasksProgressBar {
	getVals := func() (errnums int, err error) {
		errnums = 0
		if !tpm.ignoreState {
			errnums = int(tpm.stateMng.TransferFailures)
		}
		return errnums, err
	}
	pb := tpm.barMng.newCounterProgressBar(getVals, tpm.transferLabels.TransferFailures)
	return pb
}

func (tpm *TransferProgressMng) NewErrorNote() *TasksProgressBar {
	getString := func() (s string) {
		s = ""
		if tpm.stateMng.TransferFailures > 0 {
			s = tpm.transferLabels.RetryFailureContentNote
		}
		return s
	}
	pb := tpm.barMng.NewStringProgressBar("", getString)
	return pb
}

func (tpm *TransferProgressMng) WaitForPhasesGoRoutinesToFinish() {
	tpm.wg.Wait()
}

func (tpm *TransferProgressMng) WaitForReposGoRoutineToFinish() {
	tpm.reposWg.Wait()
}

func (tpm *TransferProgressMng) QuitDoubleHeadLineProgWithBar(prog *TasksWithHeadlineProg) {
	prog.headlineBar.Abort(true)
	prog.headlineBar = nil
	prog.tasksProgressBar.bar.Abort(true)
	prog.tasksProgressBar = nil
	prog.emptyLine.Abort(true)
	prog.emptyLine = nil
	tpm.barMng.barsWg.Done()
}
