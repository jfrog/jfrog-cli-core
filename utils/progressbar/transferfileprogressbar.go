package progressbar

import (
	"errors"
	"sync/atomic"
	"time"

	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"

	"github.com/gookit/color"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/vbauerster/mpb/v7"
)

// TransferProgressMng provides progress indication for the jf rt transfer-files command.
// Transferring one repository's data at a time.
type TransferProgressMng struct {
	// Determine whether the progress bar should be displayed
	shouldDisplay bool
	// Task bar with the total repositories transfer progress
	totalRepositories *tasksWithHeadlineProg
	// A bar showing the number of working transfer threads
	workingThreads *tasksProgressBar
	// Current repo progress bars
	currentRepoHeadline *mpb.Bar
	emptyLine           *mpb.Bar
	phases              []*tasksWithHeadlineProg
	// Progress bar manager
	barsMng *ProgressBarMng
	// In case of an emergency stop the transfer's progress bar will be aborted and the 'stopLine' bar will be display.
	stopLine *mpb.Bar
}

// NewTransferProgressMng creates TransferProgressMng object.
// If the progress bar shouldn't be displayed returns nil.
func NewTransferProgressMng(totalRepositories int64) (*TransferProgressMng, error) {
	mng, shouldDisplay, err := NewBarsMng()
	if !shouldDisplay || err != nil {
		return nil, err
	}
	transfer := TransferProgressMng{barsMng: mng, shouldDisplay: true}
	// Init the total repositories transfer progress bar
	transfer.totalRepositories = transfer.barsMng.NewTasksWithHeadlineProg(totalRepositories, color.Green.Render("Transferring your repositories"), false, WHITE)
	transfer.workingThreads = transfer.barsMng.NewCounterProgressBar(0, "Working threads: ")
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
		if t.workingThreads != nil {
			t.workingThreads.bar.Abort(true)
			t.workingThreads = nil
		}
		if t.currentRepoHeadline != nil {
			t.RemoveRepository()
		}
		if t.totalRepositories != nil {
			t.barsMng.quitTasksWithHeadlineProg(t.totalRepositories)
		}
		// Wait a refresh rate to make sure all aborts have finished
		time.Sleep(ProgressRefreshRate)
		// Wait for all go routines to finish before quiting
		t.barsMng.barsWg.Wait()
	} else {
		if t.stopLine != nil {
			t.stopLine.Abort(true)
			t.stopLine = nil
		}
	}
	// Close log file
	if t.barsMng.logFile != nil {
		err := corelog.CloseLogFile(t.barsMng.logFile)
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
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckError(errors.New("invalid phase id"))
	}
	if t.phases[id].tasksProgressBar.total == 0 {
		return nil
	}
	if t.ShouldDisplay() {
		t.barsMng.Increment(t.phases[id])
	}
	return nil
}

// IncrementPhaseBy increments completed tasks count for a specific phase by n.
func (t *TransferProgressMng) IncrementPhaseBy(id, n int) error {
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckError(errors.New("invalid phase id"))
	}
	if t.phases[id].tasksProgressBar.total == 0 {
		return nil
	}
	if t.phases[id].tasksProgressBar.total < t.phases[id].tasksProgressBar.tasksCount+int64(n) {
		return t.DonePhase(id)
	}
	if t.ShouldDisplay() {
		t.barsMng.IncBy(n, t.phases[id])
	}
	return nil
}

func (t *TransferProgressMng) DonePhase(id int) error {
	if id < 0 || id > len(t.phases)-1 {
		return errorutils.CheckError(errors.New("invalid phase id"))
	}
	t.barsMng.DoneTask(t.phases[id])
	return nil
}

func (t *TransferProgressMng) AddPhase1(tasksPhase1 int64) {
	t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProg(tasksPhase1, "Phase 1: Transferring all files in the repository", false, GREEN))
}

func (t *TransferProgressMng) AddPhase2(tasksPhase2 int64) {
	t.phases = append(t.phases, t.barsMng.NewTasksWithHeadlineProg(tasksPhase2, "Phase 2: Transferring newly created and modified files", false, GREEN))
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
		t.barsMng.quitTasksWithHeadlineProg(t.phases[i])
	}
	t.phases = nil
	// Wait a refresh rate to make sure all aborts have finished
	time.Sleep(ProgressRefreshRate)
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
	time.Sleep(ProgressRefreshRate)
	if t.workingThreads != nil {
		t.workingThreads.bar.Abort(true)
		t.workingThreads = nil
	}
	t.RemoveRepository()
	t.barsMng.quitTasksWithHeadlineProg(t.totalRepositories)
	t.totalRepositories = nil
	t.stopLine = t.barsMng.NewHeadlineBarWithSpinner("🛑 Gracefully stopping files transfer")
}

// Progress that includes two bars:
// 1. Headline bar
// 2. Tasks counter progress bar.
type tasksWithHeadlineProg struct {
	headlineBar      *mpb.Bar
	tasksProgressBar *tasksProgressBar
	emptyLine        *mpb.Bar
}

type generalProgressBar struct {
	bar   *mpb.Bar
	total int64
}

// tasksProgressBar counts tasks that have been completed, using a "%d/%d" format.
type tasksProgressBar struct {
	generalProgressBar
	tasksCount int64
}

// IncGeneralProgressTotalBy increments the amount of total by n.
func (p *generalProgressBar) IncGeneralProgressTotalBy(n int64) {
	atomic.AddInt64(&p.total, n)
	if p.bar != nil {
		p.bar.SetTotal(p.total, false)
	}
}

// SetGeneralProgressTotal sets the amount of total to n.
func (p *generalProgressBar) SetGeneralProgressTotal(n int64) {
	atomic.StoreInt64(&p.total, n)
	if p.bar != nil {
		p.bar.SetTotal(p.total, false)
	}
}
