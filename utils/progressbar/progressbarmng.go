package progressbar

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"golang.org/x/term"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	progressBarWidth     = 20
	longProgressBarWidth = 100
	progressRefreshRate  = 200 * time.Millisecond
)

type Color int64

const (
	WHITE Color = iota
	GREEN       = 1
)

type ProgressBarMng struct {
	// A container of all external mpb bar objects to be displayed.
	container *mpb.Progress
	// A synchronization lock object.
	barsRWMutex sync.RWMutex
	// A wait group for all progress bars.
	barsWg *sync.WaitGroup
	// The log file
	logFile *os.File
}

func NewBarsMng() (mng *ProgressBarMng, shouldInit bool, err error) {
	// Determine whether the progress bar should be displayed or not
	shouldInit, err = ShouldInitProgressBar()
	if !shouldInit || err != nil {
		return
	}
	mng = &ProgressBarMng{}
	// Init log file
	mng.logFile, err = CreateLogFile()
	if err != nil {
		return
	}
	log.Info("Log path:", mng.logFile.Name())
	log.SetLogger(log.NewLogger(corelog.GetCliLogLevel(), mng.logFile))

	mng.barsWg = new(sync.WaitGroup)
	mng.container = mpb.New(
		mpb.WithOutput(os.Stderr),
		mpb.WithWidth(longProgressBarWidth),
		mpb.WithWaitGroup(mng.barsWg),
		mpb.WithRefreshRate(progressRefreshRate))
	return
}

func (bm *ProgressBarMng) NewTasksWithHeadlineProg(totalTasks int64, headline string, spinner bool, color Color) *tasksWithHeadlineProg {
	bm.barsWg.Add(1)
	prog := tasksWithHeadlineProg{}
	if spinner {
		prog.headlineBar = bm.NewHeadlineBarWithSpinner(headline)
	} else {
		prog.headlineBar = bm.NewHeadlineBar(headline)
	}

	// If totalTasks is 0 - phase is already finished in previous run.
	if totalTasks == 0 {
		prog.tasksProgressBar = bm.NewDoneTasksProgressBar()
	} else {
		prog.tasksProgressBar = bm.NewTasksProgressBar(totalTasks, color)
	}
	prog.emptyLine = bm.NewHeadlineBar("")
	return &prog
}

func (bm *ProgressBarMng) quitTasksWithHeadlineProg(prog *tasksWithHeadlineProg) {
	prog.headlineBar.Abort(true)
	prog.tasksProgressBar.bar.Abort(true)
	prog.emptyLine.Abort(true)
	bm.barsWg.Done()
}

// NewHeadlineBar Initializes a new progress bar for headline, with an optional spinner
func (bm *ProgressBarMng) NewHeadlineBarWithSpinner(msg string) *mpb.Bar {
	return bm.container.New(1,
		mpb.SpinnerStyle("∙∙∙∙∙∙", "●∙∙∙∙∙", "∙●∙∙∙∙", "∙∙●∙∙∙", "∙∙∙●∙∙", "∙∙∙∙●∙", "∙∙∙∙∙●", "∙∙∙∙∙∙").PositionLeft(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(msg),
		),
	)
}

func (bm *ProgressBarMng) NewHeadlineBar(msg string) *mpb.Bar {
	return bm.container.Add(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(msg),
		),
	)
}

// Increment increments completed tasks count by 1.
func (bm *ProgressBarMng) Increment(prog *tasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	prog.tasksProgressBar.bar.Increment()
	prog.tasksProgressBar.tasksCount++
}

func (bm *ProgressBarMng) NewTasksProgressBar(totalTasks int64, color Color) *tasksProgressBar {
	pb := &tasksProgressBar{}
	filter := filterColor(color)
	pb.bar = bm.container.New(0,
		mpb.BarStyle().Lbound("|").Filler(filter).Tip(filter).Padding("⬛").Refiller("").Rbound("|"),
		mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Name(" Tasks: "),
			decor.CountersNoUnit("%d/%d"),
		),
	)
	pb.IncGeneralProgressTotalBy(totalTasks)
	return pb
}

func (bm *ProgressBarMng) NewDoneTasksProgressBar() *tasksProgressBar {
	pb := &tasksProgressBar{}
	pb.bar = bm.container.Add(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name("Done ✅"),
		),
	)
	return pb
}

func filterColor(color Color) (filter string) {
	switch color {
	case GREEN:
		filter = "🟩"
	case WHITE:
		filter = "⬜"
	default:
		filter = "⬜"
	}
	return
}

// TODO: duplication
// The ShouldInitProgressBar func is used to determine whether the progress bar should be displayed.
// This default implementation will init the progress bar if the following conditions are met:
// CI == false (or unset) and Stderr is a terminal.
var ShouldInitProgressBar = func() (bool, error) {
	ci, err := utils.GetBoolEnvValue(coreutils.CI, false)
	if ci || err != nil {
		return false, err
	}
	if !log.IsStdErrTerminal() {
		return false, err
	}
	err = setTerminalWidthVar()
	if err != nil {
		return false, err
	}
	return true, nil
}

var terminalWidth int

// Get terminal dimensions
func setTerminalWidthVar() error {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return err
	}
	// -5 to avoid edges
	terminalWidth = width - 5
	if terminalWidth <= 0 {
		terminalWidth = 5
	}
	return err
}

func CreateLogFile() (*os.File, error) {
	logDir, err := coreutils.CreateDirInJfrogHome(coreutils.JfrogLogsDirName)
	if err != nil {
		return nil, err
	}

	currentTime := time.Now().Format("2006-01-02.15-04-05")
	pid := os.Getpid()

	fileName := filepath.Join(logDir, "jfrog-cli."+currentTime+"."+strconv.Itoa(pid)+".log")
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	return file, nil
}
