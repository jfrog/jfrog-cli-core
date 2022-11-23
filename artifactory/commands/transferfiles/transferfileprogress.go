package transferfiles

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/vbauerster/mpb/v7"
	"time"
)

type TransferJobType string

const (
	Repositories            TransferJobType = "Repositories"
	Files                   TransferJobType = "Files"
	Storage                 TransferJobType = "Storage"
	Note                    string          = "Note: "
	RetryFailureContentNote string          = "In Phase 3 and in subsequent executions, we'll retry transferring the failed files."
)

func (tt TransferJobType) String() string {
	return string(tt)
}

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
	// A bar showing the number of transfer failures in the process
	errorBar *progressbar.TasksProgressBar
	// shows a note to the user if errors exists
	errorNote *progressbar.TasksProgressBar
	// Current repo progress bars
	currentRepoHeadline *mpb.Bar
	emptyLine           *mpb.Bar
	phases              []*progressbar.TasksWithHeadlineProg
	// Progress bar manager
	barsMng *progressbar.ProgressBarMng
	// In case of an emergency stop the transfer's progress bar will be aborted and the 'stopLine' bar will be display.
	stopLine      *mpb.Bar
	filesStatus   *int
	transferState *state.TransferStateManager
}

// NewTransferProgressMng creates TransferProgressMng object.
// If the progress bar shouldn't be displayed returns nil.
func initTransferProgressMng(allSourceLocalRepos []string, tdc *TransferFilesCommand, fileStatus int) error {
	totalRepositories := int64(len(allSourceLocalRepos))
	mng, shouldDisplay, err := progressbar.NewBarsMng()
	if !shouldDisplay || err != nil {
		return err
	}
	transfer := TransferProgressMng{barsMng: mng, shouldDisplay: true}
	transfer.transferState = tdc.stateManager
	transfer.filesStatus = &fileStatus
	// Init Progress Bars
	transfer.totalRepositories = transfer.barsMng.NewTasksWithHeadlineProg(totalRepositories, color.Green.Render("Transferring your repositories"), false, progressbar.WHITE, coreutils.RemoveEmojisIfNonSupportedTerminal("📦 "+Repositories.String()))
	transfer.totalSize = transfer.barsMng.NewDoubleValueProgressBar("🗄  "+Storage.String(), "📄 "+Files.String(), tdc.stateManager.OverallTransfer.TotalSizeBytes, nil, nil, &tdc.stateManager.OverallTransfer.TotalUnits, &tdc.stateManager.OverallTransfer.TransferredUnits, progressbar.WHITE)
	transfer.workingThreads = transfer.barsMng.NewCounterProgressBar(" 🧵 Working threads: ", 0, color.Green)
	transfer.runningTime = transfer.barsMng.NewStringProgressBar(" 🏃🏼 Running for: ", func() string {
		runningTime, isRunning, err := state.GetRunningTime()
		if err != nil || !isRunning {
			runningTime = "Running time not available"
		}
		return color.Green.Render(" " + runningTime)
	})

	transfer.speedBar = transfer.barsMng.NewStringProgressBar(" ⚡ Transfer speed: ", func() string {
		return color.Green.Render(" " + tdc.stateManager.TimeEstimationManager.GetSpeedString())
	})
	transfer.timeEstBar = transfer.barsMng.NewStringProgressBar(" ⌛ Estimated time remaining: ", func() string {
		return color.Green.Render(" " + tdc.stateManager.TimeEstimationManager.GetEstimatedRemainingTimeString())
	})

	// Init global error count for the process
	transfer.errorBar = transfer.barsMng.NewCounterProgressBar(" ❌ Transfer failures: ", 0, color.Green)
	if !tdc.ignoreState {
		numberInitialErrors, e := getRetryErrorCount(allSourceLocalRepos)
		if e != nil {
			return e
		}
		transfer.errorBar.SetGeneralProgressTotal(int64(numberInitialErrors))
	}
	transfer.errorNote = transfer.barsMng.NewStringProgressBar("", func() string {
		if transfer.errorBar.GetTotal() > 0 {
			return Note + color.Yellow.Render(RetryFailureContentNote)
		} else {
			return ""
		}
	})
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
}

// Quit terminate the TransferProgressMng process.
func (t *TransferProgressMng) Quit() error {
	if t.ShouldDisplay() {
		t.abortMetricsBars()
		if t.currentRepoHeadline != nil {
			t.RemoveRepository()
		}
		if t.totalRepositories != nil {
			t.barsMng.QuitTasksWithHeadlineProg(t.totalRepositories)
		}
		// Wait a refresh rate to make sure all aborts have finished
		time.Sleep(progressbar.ProgressRefreshRate)
		// Wait for all go routines to finish before quiting
		t.barsMng.GetBarsWg().Wait()
	} else {
		if t.stopLine != nil {
			t.stopLine.Abort(true)
			t.stopLine = nil
		}
	}

	// Close log file
	if t.barsMng.GetLogFile() != nil {
		err := corelog.CloseLogFile(t.barsMng.GetLogFile())
		if err != nil {
			return err
		}
		// Set back the default logger
		corelog.SetDefaultLogger()
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

func (t *TransferProgressMng) AddPhase1(storage int64, skip bool) error {
	_, _, totalFiles, transferredFiles, err := t.transferState.GetStorageAndFilesRepoPointers(t.transferState.CurrentRepo, api.Phase1)
	if err != nil {
		return err
	}
	if skip {
		t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProg(0, "Phase 1: Transferring all files in the repository", false, progressbar.GREEN, ""))
	}

	if !skip {
		t.phases = append(t.phases, t.barsMng.NewHeadLineDoubleValProgBar("Phase 1: Transferring all files in the repository", "🗄  "+Storage.String(), "📄 "+Files.String(), storage, nil, nil, totalFiles, transferredFiles, progressbar.GREEN))
	}
	return nil
}

func (t *TransferProgressMng) AddPhase2() error {
	totalDiffStorage, totalUploadedDiffStorage, totalDiffFiles, totalUploadedDiffFiles, err := t.transferState.GetStorageAndFilesRepoPointers(t.transferState.CurrentRepo, api.Phase2)
	if err != nil {
		return err
	}
	t.phases = append(t.phases, t.barsMng.NewHeadLineDoubleValProgBar("Phase 2: Transferring newly created and modified files", "🗄  Diff Storage", "📄 Diff Files", 0, totalDiffStorage, totalUploadedDiffStorage, totalDiffFiles, totalUploadedDiffFiles, progressbar.GREEN))
	return nil
}

func (t *TransferProgressMng) AddPhase3(totalStorage int64) error {
	_, _, totalFailedFiles, totalUploadedFailedFiles, err := t.transferState.GetStorageAndFilesRepoPointers(t.transferState.CurrentRepo, api.Phase3)
	if err != nil {
		return err
	}
	t.phases = append(t.phases, t.barsMng.NewHeadLineDoubleValProgBar("Phase 3: Retrying transfer failures", "🗄  Failed Storage", "📄 Failed Files", totalStorage, nil, nil, totalFailedFiles, totalUploadedFailedFiles, progressbar.GREEN))
	return nil
}

func (t *TransferProgressMng) RemoveRepository() {
	if t.currentRepoHeadline == nil {
		return
	}
	// Increment total repositories progress bar
	t.barsMng.Increment(t.totalRepositories)

	// Abort all current repository's bars
	t.currentRepoHeadline.Abort(true)
	t.currentRepoHeadline = nil
	t.emptyLine.Abort(true)
	t.emptyLine = nil
	// Abort all phases bars
	for i := 0; i < len(t.phases); i++ {
		t.barsMng.QuitTasksWithHeadlineProg(t.phases[i])
	}
	t.phases = nil
	// Wait a refresh rate to make sure all aborts have finished
	time.Sleep(progressbar.ProgressRefreshRate)
}

func (t *TransferProgressMng) changeNumberOfFailuresBy(n int) {
	if t.ShouldDisplay() {
		diff := int64(n)
		t.errorBar.SetGeneralProgressTotal(t.errorBar.GetTotal() + diff)
	}
}

func (t *TransferProgressMng) SetRunningThreads(n int) {
	if t.ShouldDisplay() {
		t.workingThreads.SetGeneralProgressTotal(int64(n))
	}
}

func (t *TransferProgressMng) increaseTotalSize(n int) {
	if t.ShouldDisplay() {
		t.totalSize.GetBar().IncrBy(n)
	}
}

func (t *TransferProgressMng) StopGracefully() {
	if !t.ShouldDisplay() {
		return
	}
	t.shouldDisplay = false
	// Wait a refresh rate to make sure all 'increase' operations have finished before aborting all bars
	time.Sleep(progressbar.ProgressRefreshRate)
	t.abortMetricsBars()
	t.RemoveRepository()
	t.barsMng.QuitTasksWithHeadlineProg(t.totalRepositories)
	t.totalRepositories = nil
	t.stopLine = t.barsMng.NewHeadlineBarWithSpinner(coreutils.RemoveEmojisIfNonSupportedTerminal("🛑 Gracefully stopping files transfer"))
}

func (t *TransferProgressMng) abortMetricsBars() {
	for _, barPtr := range []*progressbar.TasksProgressBar{t.runningTime, t.workingThreads, t.errorBar, t.speedBar, t.timeEstBar, t.totalSize} {
		if barPtr != nil {
			barPtr.GetBar().Abort(true)
			barPtr = nil
		}
	}
}
