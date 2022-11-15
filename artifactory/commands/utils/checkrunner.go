package utils

import (
	"context"
	"fmt"
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// PreCheck Is interface for a check on an Artifactory server
type PreCheck interface {
	// Name - Describes the check
	Name() string
	// Execute the check, return true if passed
	ExecuteCheck(args RunArguments) (bool, error)
}

// Default struct for small checks
type functionPreCheck struct {
	name  string
	check func(args RunArguments) (bool, error)
}

func (a functionPreCheck) Name() string {
	return a.name
}

func (a functionPreCheck) ExecuteCheck(args RunArguments) (bool, error) {
	return a.check(args)
}

// NewCheck - Creates a check base on a given function and name
func NewCheck(name string, check func(args RunArguments) (bool, error)) PreCheck {
	return functionPreCheck{
		name:  name,
		check: check,
	}
}

// PreCheckRunner - Manages and runs the pre-checks
type PreCheckRunner struct {
	// All the checks the runner need to preform
	checks []PreCheck
	// The status of the preformed checks on the current run
	status *RunStatus
	// Manage all the displayed progress of the run
	displayBar *RunnerProgressBar
}

// Manage all the displayed progress of the run
type RunnerProgressBar struct {
	manager        *progressbar.ProgressBarMng
	currentDisplay *progressbar.TasksProgressBar
}

// RunStatus - The status of the preformed checks in a run
type RunStatus struct {
	failures     uint
	successes    uint
	currentCheck string
	startTime    time.Time
}

// RunArguments - The arguments of the run that is passed to the checks
type RunArguments struct {
	Context       context.Context
	ServerDetails *config.ServerDetails
	ProgressMng   *progressbar.ProgressBarMng
}

// Creates a new empty runner
func NewPreChecksRunner() *PreCheckRunner {
	runner := &PreCheckRunner{}
	return runner
}

// Add a check to the runner
func (pcr *PreCheckRunner) AddCheck(check PreCheck) {
	if check == nil {
		return
	}
	pcr.checks = append(pcr.checks, check)
}

// Initialize a progress bar for running pre-checks
func (pcr *PreCheckRunner) initProgressBar(status *RunStatus) (runnerProgress *RunnerProgressBar, err error) {
	mng, shouldDisplay, err := progressbar.NewBarsMng()
	if !shouldDisplay || err != nil {
		return
	}
	runnerProgress = &RunnerProgressBar{manager: mng}
	// The current check that is running
	runnerProgress.currentDisplay = mng.NewStringProgressBar("Running check: ", func() string {
		return color.Green.Render(status.currentCheck)
	})
	return
}

// Run all the checks and display the process
func (pcr *PreCheckRunner) Run(context context.Context, serverDetails *config.ServerDetails) (err error) {
	log.Info(coreutils.PrintTitle(fmt.Sprintf("Running %d checks.", len(pcr.checks))))
	args := RunArguments{Context: context, ServerDetails: serverDetails}
	pcr.status = &RunStatus{startTime: time.Now()}
	// Progress display
	if pcr.displayBar, err = pcr.initProgressBar(pcr.status); err != nil {
		return
	}
	if pcr.displayBar != nil {
		args.ProgressMng = pcr.displayBar.manager
	}
	// Execute checks
	defer func() {
		if e := pcr.cleanup(); e != nil && err == nil {
			err = e
		}
	}()
	var checkPassed bool
	for i, check := range pcr.checks {
		pcr.prepare(i, check)
		if checkPassed, err = check.ExecuteCheck(args); err != nil {
			pcr.finish(check.Name(), false)
			return
		}
		pcr.finish(check.Name(), checkPassed)
	}
	return
}

// Update the runner before a check
func (pcr *PreCheckRunner) prepare(checkNumber int, check PreCheck) {
	log.Info(fmt.Sprintf("== Running check (%d) '%s' ======", checkNumber+1, check.Name()))
	pcr.status.currentCheck = check.Name()
}

// Update the runner after a check
func (pcr *PreCheckRunner) finish(checkName string, passed bool) {
	// Update status
	checkStatus := "Fail"
	if passed {
		checkStatus = "Success"
		pcr.status.successes++
	} else {
		pcr.status.failures++
	}
	// Update progress
	log.Info(fmt.Sprintf("Check '%s' is done with status %s", checkName, checkStatus))
}

// Clean up when the run ends
func (pcr *PreCheckRunner) cleanup() (err error) {
	// Quit progress bar
	if pcr.displayBar != nil {
		// Quit text - current check
		pcr.displayBar.currentDisplay.GetBar().Abort(true)
		pcr.displayBar.currentDisplay = nil
		// Wait a refresh rate to make sure all aborts have finished
		time.Sleep(progressbar.ProgressRefreshRate)
		// Wait for all go routines to finish before quiting
		pcr.displayBar.manager.GetBarsWg().Wait()
		// Close log file
		if pcr.displayBar.manager.GetLogFile() != nil {
			if err = corelog.CloseLogFile(pcr.displayBar.manager.GetLogFile()); err != nil {
				return
			}
		}
	}
	// Notify on final status of the run
	if pcr.status.failures == 0 && len(pcr.checks) == int(pcr.status.successes+pcr.status.failures) && err == nil {
		log.Info(coreutils.PrintTitle(fmt.Sprintf("All the checks passed 🐸 (elapsed time %s).", time.Since(pcr.status.startTime))))
	} else {
		log.Error(coreutils.PrintTitle(fmt.Sprintf("%d/%d checks passed (elapsed time %s), check the log for more information.",
			pcr.status.successes,
			pcr.status.successes+pcr.status.failures,
			time.Since(pcr.status.startTime),
		)))
	}

	return
}
