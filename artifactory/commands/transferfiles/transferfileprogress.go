package transferfiles

import (
	"github.com/gookit/color"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/vbauerster/mpb/v7"
	"time"
)

type TransferJobType string

const (
	Repositories TransferJobType = "Repositories"
	Files        TransferJobType = "Files"
	TimeSlots    TransferJobType = "Time Slots"
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
	// A bar showing the number of working transfer threads
	workingThreads *progressbar.TasksProgressBar
	// A bar showing the speed of the data transfer
	speedBar *progressbar.TasksProgressBar
	// A bar showing the estimated remaining time for the transfer
	timeEstBar *progressbar.TasksProgressBar
	// Current repo progress bars
	currentRepoHeadline *mpb.Bar
	emptyLine           *mpb.Bar
	phases              []*progressbar.TasksWithHeadlineProg
	// Progress bar manager
	barsMng *progressbar.ProgressBarMng
	// In case of an emergency stop the transfer's progress bar will be aborted and the 'stopLine' bar will be display.
	stopLine *mpb.Bar
}

// NewTransferProgressMng creates TransferProgressMng object.
// If the progress bar shouldn't be displayed returns nil.
func NewTransferProgressMng(totalRepositories int64, timeEstMng *timeEstimationManager) (*TransferProgressMng, error) {
	mng, shouldDisplay, err := progressbar.NewBarsMng()
	if !shouldDisplay || err != nil {
		return nil, err
	}
	transfer := TransferProgressMng{barsMng: mng, shouldDisplay: true}
	// Init the total repositories transfer progress bar
	transfer.totalRepositories = transfer.barsMng.NewTasksWithHeadlineProg(totalRepositories, color.Green.Render("Transferring your repositories"), false, progressbar.WHITE, Repositories.String())
	transfer.workingThreads = transfer.barsMng.NewCounterProgressBar(0, "Working threads: ")
	if timeEstMng != nil {
		transfer.speedBar = transfer.barsMng.NewStringProgressBar("Transfer speed: ", func() string {
			return color.Green.Render(timeEstMng.getSpeedString())
		})
		transfer.timeEstBar = transfer.barsMng.NewStringProgressBar("Time remaining: ", func() string {
			return color.Green.Render(timeEstMng.getEstimatedRemainingTimeString())
		})
	}
	return &transfer, nil
}

// NewRepository adds new repository's progress details.
// Aborting previous repository if exists.
func (t *TransferProgressMng) NewRepository(name string) {
	// Abort previous repository before creating the new one
	if t.currentRepoHeadline != nil {
		t.RemoveRepository()
	}
	t.currentRepoHeadline = t.barsMng.NewHeadlineBarWithSpinner("Current repository: " + color.Green.Render(name))
	t.emptyLine = t.barsMng.NewHeadlineBar("")
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
func (t *TransferProgressMng) IncrementPhaseBy(id, n int) error {
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

func (t *TransferProgressMng) AddPhase1(tasksPhase1 int64) {
	t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProg(tasksPhase1, "Phase 1: Transferring all files in the repository", false, progressbar.GREEN, Files.String()))
}

func (t *TransferProgressMng) AddPhase2(tasksPhase2 int64) {
	t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProg(tasksPhase2, "Phase 2: Transferring newly created and modified files", false, progressbar.GREEN, TimeSlots.String()))
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

func (t *TransferProgressMng) SetRunningThreads(n int) {
	if t.ShouldDisplay() {
		t.workingThreads.SetGeneralProgressTotal(int64(n))
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
	t.stopLine = t.barsMng.NewHeadlineBarWithSpinner("🛑 Gracefully stopping files transfer")
}

func (t *TransferProgressMng) abortMetricsBars() {
	if t.workingThreads != nil {
		t.workingThreads.GetBar().Abort(true)
		t.workingThreads = nil
	}
	if t.speedBar != nil {
		t.speedBar.GetBar().Abort(true)
		t.speedBar = nil
	}
	if t.timeEstBar != nil {
		t.timeEstBar.GetBar().Abort(true)
		t.timeEstBar = nil
	}
}
