package transferfiles

import (
	"time"

	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreLog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/vbauerster/mpb/v7"
)

const phase1HeadLine = "Phase 1: Transferring all files in the repository"

// TransferProgressMng provides progress indication for the jf rt transfer-files command.
// Transferring one repository's data at a time.
type TransferProgressMng struct {
	// Determine whether the progress bar should be displayed
	shouldDisplay bool
	// Task bar with the total repositories transfer progress
	totalRepositories *progressbar.TasksWithHeadlineProg
	// A bar showing the running time
	runningTime *progressbar.TasksProgressBar
	// A bar showing the remaining storage to transfer in the process
	totalSize *progressbar.TasksProgressBar
	// A bar showing the number of working transfer threads
	workingThreads *progressbar.TasksProgressBar
	// A bar showing the speed of the data transfer
	speedBar *progressbar.TasksProgressBar
	// A bar showing the estimated remaining time for the transfer
	timeEstBar *progressbar.TasksProgressBar
	// A bar showing the number of visited folders
	visitedFoldersBar *progressbar.TasksProgressBar
	// A bar showing the number of delayed artifacts in the process
	delayedBar *progressbar.TasksProgressBar
	// A bar showing the number of transfer failures in the process
	errorBar *progressbar.TasksProgressBar
	// Current repo progress bars
	currentRepoHeadline *mpb.Bar
	emptyLine           *mpb.Bar
	phases              []*progressbar.TasksWithHeadlineProg
	// Progress bar manager
	barsMng *progressbar.ProgressBarMng
	// Transfer progress bar manager
	transferMng *progressbar.TransferProgressMng
	// In case of an emergency stop the transfer's progress bar will be aborted and the 'stopLine' bar will be display.
	stopLine *mpb.Bar
	// Progress bar labels
	filesStatus   *int
	transferState *state.TransferStateManager
	windows       bool
}

// NewTransferProgressMng creates TransferProgressMng object.
// If the progress bar shouldn't be displayed returns nil.
func initTransferProgressMng(allSourceLocalRepos []string, tdc *TransferFilesCommand, fileStatus int) error {
	// Init the transfer progress bar manager
	trmng, shouldDisplay, err := progressbar.InitTransferProgressBarMng(tdc.stateManager, allSourceLocalRepos)
	if !shouldDisplay || err != nil {
		return err
	}
	transfer := TransferProgressMng{barsMng: trmng.GetBarMng(), shouldDisplay: true, transferMng: trmng}
	transfer.transferState = tdc.stateManager
	transfer.filesStatus = &fileStatus
	transfer.windows = coreutils.IsWindows()
	transfer.transferMng.StopCurrentRepoProgressBars(false)
	// Init Progress Bars
	transfer.totalRepositories = transfer.transferMng.NewRepositoriesProgressBar()
	transfer.totalSize = transfer.transferMng.NewGeneralProgBar()
	transfer.workingThreads = transfer.transferMng.NewWorkingThreadsProg()
	transfer.runningTime = transfer.transferMng.NewRunningTimeProgressBar()
	transfer.speedBar = transfer.transferMng.NewSpeedProgBar()
	transfer.timeEstBar = transfer.transferMng.NewTimeEstBar()
	// Init global error count for the process
	transfer.errorBar = transfer.transferMng.NewErrorBar()
	tdc.progressbar = &transfer
	return nil
}

// NewRepository adds new repository's progress details.
// Aborting previous repository if exists.
func (t *TransferProgressMng) NewRepository(name string) {
	// Abort previous repository before creating the new one
	if t.currentRepoHeadline != nil {
		t.RemoveRepository()
	}
	t.emptyLine = t.barsMng.NewHeadlineBar("")
	t.currentRepoHeadline = t.barsMng.NewHeadlineBarWithSpinner("Current repository: " + color.Green.Render(name))
	t.visitedFoldersBar = t.transferMng.NewVisitedFoldersBar()
	t.delayedBar = t.transferMng.NewDelayedBar()
	t.transferMng.StopCurrentRepoProgressBars(false)
}

// Quit terminate the TransferProgressMng process.
func (t *TransferProgressMng) Quit() error {
	t.transferMng.StopCurrentRepoProgressBars(true)
	t.transferMng.StopGlobalProgressBars()
	if t.ShouldDisplay() {
		t.abortMetricsBars()
		if t.currentRepoHeadline != nil {
			t.RemoveRepository()
		}
		if t.totalRepositories != nil {
			t.barsMng.QuitTasksWithHeadlineProgressBar(t.totalRepositories)
		}
		// Wait a few refresh rates to make sure all aborts have finished.
		time.Sleep(progressbar.ProgressRefreshRate * 3)
		// Wait for all go routines to finish before quiting
		t.transferMng.WaitForReposGoRoutineToFinish()
		t.barsMng.GetBarsWg().Wait()
	} else if t.stopLine != nil {
		t.stopLine.Abort(true)
		t.stopLine = nil
	}

	// Close log file
	if t.barsMng.GetLogFile() != nil {
		err := coreLog.CloseLogFile(t.barsMng.GetLogFile())
		if err != nil {
			return err
		}
		// Set back the default logger
		coreLog.SetDefaultLogger()
	}
	return nil
}

func (t *TransferProgressMng) ShouldDisplay() bool {
	return t.shouldDisplay
}

// IncrementPhase increments completed tasks count for a specific phase by 1.
func (t *TransferProgressMng) IncrementPhase(id int) error {
	if len(t.phases) == 0 {
		// Progress bar was terminated
		return nil
	}
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckErrorf("IncrementPhase: invalid phase id %d", id)
	}
	if t.phases[id].GetTasksProgressBar().GetTotal() == 0 {
		return nil
	}
	if t.ShouldDisplay() {
		t.barsMng.Increment(t.phases[id])
	}
	return nil
}

// IncrementPhaseBy increments completed tasks count for a specific phase by n.
func (t *TransferProgressMng) IncrementPhaseBy(id int, n int) error {
	if len(t.phases) == 0 {
		// Progress bar was terminated
		return nil
	}
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckErrorf("IncrementPhaseBy: invalid phase id %d", id)
	}
	if t.phases[id].GetTasksProgressBar().GetTotal() == 0 {
		return nil
	}
	if t.phases[id].GetTasksProgressBar().GetTotal() < t.phases[id].GetTasksProgressBar().GetTasksCount()+int64(n) {
		t.barsMng.IncBy(n, t.phases[id])
		return t.DonePhase(id)
	}
	if t.ShouldDisplay() {
		t.barsMng.IncBy(n, t.phases[id])
	}
	return nil
}

func (t *TransferProgressMng) DonePhase(id int) error {
	if len(t.phases) == 0 {
		// Progress bar was terminated
		return nil
	}
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckErrorf("DonePhase: invalid phase id %d", id)
	}
	t.barsMng.DoneTask(t.phases[id])
	return nil
}

func (t *TransferProgressMng) AddPhase1(skip bool) {
	if skip {
		t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProgressBar(0, phase1HeadLine, false, t.windows, ""))
	} else {
		bar2 := t.transferMng.NewPhase1ProgressBar()
		t.phases = append(t.phases, bar2)
	}
}

func (t *TransferProgressMng) AddPhase2() {
	bar := t.transferMng.NewPhase2ProgressBar()
	t.phases = append(t.phases, bar)
}

func (t *TransferProgressMng) AddPhase3() {
	bar := t.transferMng.NewPhase3ProgressBar()
	t.phases = append(t.phases, bar)
}

func (t *TransferProgressMng) RemoveRepository() {
	if t.currentRepoHeadline == nil {
		return
	}
	// Abort all current repository's bars
	t.currentRepoHeadline.Abort(true)
	t.currentRepoHeadline = nil
	t.visitedFoldersBar.GetBar().Abort(true)
	t.delayedBar.GetBar().Abort(true)
	t.emptyLine.Abort(true)
	t.emptyLine = nil
	// Abort all phases bars
	for i := 0; i < len(t.phases); i++ {
		t.transferMng.QuitDoubleHeadLineProgWithBar(t.phases[i])
	}
	t.transferMng.StopCurrentRepoProgressBars(true)
	t.transferMng.WaitForPhasesGoRoutinesToFinish()
	t.phases = nil

	// Wait a refresh rate to make sure all aborts have finished
	time.Sleep(progressbar.ProgressRefreshRate)
}

func (t *TransferProgressMng) incNumberOfVisitedFolders() {
	if t.ShouldDisplay() {
		t.visitedFoldersBar.SetGeneralProgressTotal(t.visitedFoldersBar.GetTotal() + 1)
	}
}

func (t *TransferProgressMng) changeNumberOfDelayedFiles(n int) {
	if t.ShouldDisplay() {
		diff := int64(n)
		t.delayedBar.SetGeneralProgressTotal(t.delayedBar.GetTotal() + diff)
	}
}

func (t *TransferProgressMng) changeNumberOfFailuresBy(n int) {
	if t.ShouldDisplay() {
		diff := int64(n)
		t.errorBar.SetGeneralProgressTotal(t.errorBar.GetTotal() + diff)
	}
}

func (t *TransferProgressMng) StopGracefully() {
	if !t.ShouldDisplay() {
		return
	}
	t.shouldDisplay = false
	// Wait a refresh rate to make sure all 'increase' operations have finished before aborting all bars
	time.Sleep(progressbar.ProgressRefreshRate)
	t.transferMng.StopGlobalProgressBars()
	t.abortMetricsBars()
	t.RemoveRepository()
	t.transferMng.WaitForReposGoRoutineToFinish()
	t.barsMng.QuitTasksWithHeadlineProgressBar(t.totalRepositories)
	t.totalRepositories = nil
	t.stopLine = t.barsMng.NewHeadlineBarWithSpinner(coreutils.RemoveEmojisIfNonSupportedTerminal("🛑 Gracefully stopping files transfer"))
}

func (t *TransferProgressMng) abortMetricsBars() {
	for _, barPtr := range []*progressbar.TasksProgressBar{t.runningTime, t.workingThreads, t.visitedFoldersBar, t.delayedBar, t.errorBar, t.speedBar, t.timeEstBar, t.totalSize} {
		if barPtr != nil {
			barPtr.GetBar().Abort(true)
		}
	}
}
