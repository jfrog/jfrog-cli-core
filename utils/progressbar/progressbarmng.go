package progressbar

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
	golangLog "log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ProgressBarWidth     = 20
	longProgressBarWidth = 100
	ProgressRefreshRate  = 200 * time.Millisecond
)

var terminalWidth int

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
	mng.logFile, err = corelog.CreateLogFile()
	if err != nil {
		return
	}
	log.Info("Log path:", mng.logFile.Name())
	log.SetLogger(log.NewLoggerWithFlags(corelog.GetCliLogLevel(), mng.logFile, golangLog.Ldate|golangLog.Ltime|golangLog.Lmsgprefix))

	mng.barsWg = new(sync.WaitGroup)
	mng.container = mpb.New(
		mpb.WithOutput(os.Stderr),
		mpb.WithWidth(longProgressBarWidth),
		mpb.WithWaitGroup(mng.barsWg),
		mpb.WithRefreshRate(ProgressRefreshRate))
	return
}

// Initializing a new Tasks with headline progress bar
// Initialize a progress bar that can show the status of two different values, and a headline above it
func (bm *ProgressBarMng) newDoubleHeadLineProgressBar(headline, val1HeadLine, val2HeadLine string, getVal func() (firstNumerator, firstDenominator, secondNumerator, secondDenominator *int64, err error)) *TasksWithHeadlineProg {
	bm.barsWg.Add(1)
	prog := TasksWithHeadlineProg{}
	prog.headlineBar = bm.NewHeadlineBar(headline)
	prog.tasksProgressBar = bm.newDoubleValueProgressBar(getVal, val1HeadLine, val2HeadLine)
	prog.emptyLine = bm.NewHeadlineBar("")

	return &prog
}

// Initialize a progress bar that can show the status of two different values
func (bm *ProgressBarMng) newDoubleValueProgressBar(getVal func() (firstNumerator, firstDenominator, secondNumerator, secondDenominator *int64, err error), firstValueLine, secondValueLine string) *TasksProgressBar {
	pb := TasksProgressBar{}
	pb.bar = bm.container.New(0,
		SingleTaskBarStyle(),
		mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Name(" "+firstValueLine+": "),
			decor.Any(func(statistics decor.Statistics) string {
				firstNumerator, firstDenominator, _, _, err := getVal()
				if err != nil {
					log.Error(err)
				}
				s1 := servicesUtils.ConvertIntToStorageSizeString(*firstNumerator)
				s2 := servicesUtils.ConvertIntToStorageSizeString(*firstDenominator)
				return color.Green.Render(s1 + "/" + s2)
			}), decor.Name(" "+secondValueLine+": "), decor.Any(func(statistics decor.Statistics) string {
				_, _, secondNumerator, secondDenominator, err := getVal()
				if err != nil {
					log.Error(err)
				}
				s1 := strconv.Itoa(int(*secondNumerator))
				s2 := strconv.Itoa(int(*secondDenominator))
				return color.Green.Render(s1 + "/" + s2)
			}),
		),
	)
	return &pb
}

// Initialize a regular tasks progress bar, with a headline above it
func (bm *ProgressBarMng) newHeadlineTaskProgressBar(getVal func() (numerator, denominator *int64), headLine, valHeadLine string) *TasksWithHeadlineProg {
	bm.barsWg.Add(1)
	prog := TasksWithHeadlineProg{}
	prog.headlineBar = bm.NewHeadlineBar(headLine)
	prog.tasksProgressBar = bm.newTasksProgressBar(getVal, valHeadLine)
	prog.emptyLine = bm.NewHeadlineBar("")
	return &prog
}

// Initialize a regular tasks progress bar, with a headline above it
func (bm *ProgressBarMng) NewTasksWithHeadlineProgressBar(totalTasks int64, headline string, spinner bool, taskType string) *TasksWithHeadlineProg {
	bm.barsWg.Add(1)
	prog := TasksWithHeadlineProg{}
	if spinner {
		prog.headlineBar = bm.NewHeadlineBarWithSpinner(headline)
	} else {
		prog.headlineBar = bm.NewHeadlineBar(headline)
	}
	// If totalTasks is 0 - phase is already finished in previous run.
	if totalTasks == 0 {
		prog.tasksProgressBar = bm.newDoneTasksProgressBar()
	} else {
		prog.tasksProgressBar = bm.NewTasksProgressBar(totalTasks, taskType)
	}
	prog.emptyLine = bm.NewHeadlineBar("")
	return &prog
}

func (bm *ProgressBarMng) QuitTasksWithHeadlineProgressBar(prog *TasksWithHeadlineProg) {
	prog.headlineBar.Abort(true)
	prog.headlineBar = nil
	prog.tasksProgressBar.bar.Abort(true)
	prog.tasksProgressBar = nil
	prog.emptyLine.Abort(true)
	prog.emptyLine = nil
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

func (bm *ProgressBarMng) NewUpdatableHeadlineBarWithSpinner(updateFn func() string) *mpb.Bar {
	return bm.container.New(1,
		mpb.SpinnerStyle("∙∙∙∙∙∙", "●∙∙∙∙∙", "∙●∙∙∙∙", "∙∙●∙∙∙", "∙∙∙●∙∙", "∙∙∙∙●∙", "∙∙∙∙∙●", "∙∙∙∙∙∙").PositionLeft(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Any(func(statistics decor.Statistics) string {
				return updateFn()
			}),
		),
	)
}

func (bm *ProgressBarMng) NewHeadlineBar(msg string) *mpb.Bar {
	return bm.container.MustAdd(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(msg),
		),
	)
}

// Increment increments completed tasks count by 1.
func (bm *ProgressBarMng) Increment(prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	prog.tasksProgressBar.bar.Increment()
	prog.tasksProgressBar.tasksCount++
}

// Increment increments completed tasks count by n.
func (bm *ProgressBarMng) IncBy(n int, prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	prog.tasksProgressBar.bar.IncrBy(n)
	prog.tasksProgressBar.tasksCount += int64(n)
}

// DoneTask increase tasks counter to the number of totalTasks.
func (bm *ProgressBarMng) DoneTask(prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	diff := prog.tasksProgressBar.total - prog.tasksProgressBar.tasksCount
	// diff is int64, but we can increase the progress up to math.MaxInt in a time
	for ; diff > math.MaxInt; diff -= math.MaxInt {
		prog.tasksProgressBar.bar.IncrBy(math.MaxInt)
	}
	prog.tasksProgressBar.bar.IncrBy(int(diff))
}

func (bm *ProgressBarMng) NewTasksProgressBar(totalTasks int64, taskType string) *TasksProgressBar {
	pb := &TasksProgressBar{}
	if taskType == "" {
		taskType = "Tasks"
	}
	pb.bar = bm.container.New(0,
		SingleTaskBarStyle(),
		mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Name(" "+taskType+": "),
			decor.CountersNoUnit(getRenderedFormattedCounters("%d")),
		),
	)
	pb.IncGeneralProgressTotalBy(totalTasks)
	return pb
}

func (bm *ProgressBarMng) newTasksProgressBar(getVal func() (numerator, denominator *int64), headLine string) *TasksProgressBar {
	pb := &TasksProgressBar{}
	numerator, denominator := getVal()
	pb.bar = bm.container.New(0,
		SingleTaskBarStyle(),
		mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Name(" "+headLine+": "),
			decor.Any(func(statistics decor.Statistics) string {
				numeratorString := strconv.Itoa(int(*numerator))
				denominatorString := strconv.Itoa(int(*denominator))
				return color.Green.Render(numeratorString + "/" + denominatorString)
			}),
		),
	)
	return pb
}

// Initializing a counter progress bar
func (bm *ProgressBarMng) newCounterProgressBar(getVal func() (value int, err error), headLine string, counterDescription decor.Decorator) *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.MustAdd(0,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(headLine),
			decor.Any(func(decor.Statistics) string {
				value, err := getVal()
				if err != nil {
					log.Error(err)
				}
				s1 := strconv.Itoa(value)
				return color.Green.Render(s1)
			}),
		),
		mpb.AppendDecorators(counterDescription),
	)
	return pb
}

// Initializing a progress bar that shows Done status
func (bm *ProgressBarMng) newDoneTasksProgressBar() *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.MustAdd(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name("Done ✅"),
		),
	)
	return pb
}

func (bm *ProgressBarMng) NewStringProgressBar(headline string, updateFn func() string) *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.MustAdd(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(headline), decor.Any(func(statistics decor.Statistics) string {
				return updateFn()
			}),
		),
	)
	return pb
}

func getRenderedFormattedCounters(formatDirective string) string {
	return color.Green.Render(strings.Join([]string{formatDirective, formatDirective}, "/"))
}

func (bm *ProgressBarMng) GetBarsWg() *sync.WaitGroup {
	return bm.barsWg
}

func (bm *ProgressBarMng) GetLogFile() *os.File {
	return bm.logFile
}

func SingleTaskBarStyle() mpb.BarStyleComposer {
	return barStyle(false)
}

func GeneralBarStyle() mpb.BarStyleComposer {
	return barStyle(true)
}

func barStyle(isGeneral bool) mpb.BarStyleComposer {
	padding := ".."
	filler := "●"
	if !coreutils.IsWindows() {
		padding = "  "
		if isGeneral {
			filler = "🟦"
		} else {
			filler = "🟩"
		}
	}
	return mpb.BarStyle().Lbound("").Filler(filler).Tip(filler).Padding(padding).Refiller("").Rbound("")
}

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
	err = setTerminalWidth()
	if err != nil {
		return false, err
	}
	return true, nil
}

func setTerminalWidth() error {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return errorutils.CheckError(err)
	}
	// -5 to avoid edges
	terminalWidth = width - 5
	if terminalWidth <= 0 {
		terminalWidth = 5
	}
	return err
}

func GetTerminalWidth() int {
	return terminalWidth
}
