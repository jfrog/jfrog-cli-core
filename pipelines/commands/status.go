package commands

import (
	"context"
	"fmt"
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"time"
)

type StatusCommand struct {
	// Server details with pipelines server URl and authentication
	serverDetails *config.ServerDetails
	// Branch name for applying filter on pipeline statuses
	branch string
	// Pipeline name to apply filter on pipeline statuses
	pipelineName string
	// Notify used for determining for continuous monitoring of status and notifying
	notify        bool
	isMultiBranch bool
}

const (
	PipelineName                      = "PipelineName :"
	Branch                            = "Branch :"
	Run                               = "Run :"
	Duration                          = "Duration :"
	StatusLabel                       = "Status :"
	MaxRetries                        = 1500
	MinimumIntervalRetriesInMilliSecs = 5000
)

func NewStatusCommand() *StatusCommand {
	return &StatusCommand{}
}

func (sc *StatusCommand) ServerDetails() (*config.ServerDetails, error) {
	return sc.serverDetails, nil
}

func (sc *StatusCommand) SetServerDetails(serverDetails *config.ServerDetails) *StatusCommand {
	sc.serverDetails = serverDetails
	return sc
}

func (sc *StatusCommand) CommandName() string {
	return "pl_status"
}

func (sc *StatusCommand) SetBranch(br string) *StatusCommand {
	sc.branch = br
	return sc
}

func (sc *StatusCommand) SetPipeline(pl string) *StatusCommand {
	sc.pipelineName = pl
	return sc
}

func (sc *StatusCommand) SetNotify(nf bool) *StatusCommand {
	sc.notify = nf
	return sc
}

func (sc *StatusCommand) SetMultiBranch(multiBranch bool) *StatusCommand {
	sc.isMultiBranch = multiBranch
	return sc
}

func (sc *StatusCommand) Run() error {
	// Create service manager to fetch run status
	serviceManager, err := manager.CreateServiceManager(sc.serverDetails)
	if err != nil {
		return err
	}

	// Get pipeline status using branch name, pipelines name and whether it is multi-branch
	matchingPipes, err := serviceManager.GetPipelineRunStatusByBranch(sc.branch, sc.pipelineName, sc.isMultiBranch)
	if err != nil {
		return err
	}
	var res string
	for i := range matchingPipes.Pipelines {
		pipe := matchingPipes.Pipelines[i]
		// Eliminate pipelines which have not been run
		if pipe.LatestRunID != 0 {
			// When notification option is selected use this flow to notify
			if sc.pipelineName != "" && sc.notify {
				err := monitorStatusAndNotify(context.Background(), serviceManager, sc.branch, sc.pipelineName, sc.isMultiBranch)
				if err != nil {
					return err
				}
			} else {
				respStatus, colorCode, duration := getPipelineStatusAndColorCode(&pipe)
				res = res + colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", PipelineName,
					pipe.Name, Branch, pipe.PipelineSourceBranch, Run, pipe.Run.RunNumber, Duration,
					duration, StatusLabel, string(respStatus))
			}
		}
	}
	log.Output(res)
	return nil
}

// getPipelineStatusAndColorCode from received pipeline statusCode
// return PipelineStatus - pipeline status string conversion from statusCode
// colorCode - color to be used for text formatting
// duration - duration for the pipeline in seconds
func getPipelineStatusAndColorCode(pipeline *services.Pipelines) (pipelineStatus status.PipelineStatus, colorCode color.Color, duration string) {
	pipelineStatus = status.GetPipelineStatus(pipeline.Run.StatusCode)
	colorCode = status.GetStatusColorCode(pipelineStatus)
	durationSeconds := pipeline.Run.DurationSeconds
	if durationSeconds == 0 {
		// Calculate the duration time by differentiating created time from the current time
		durationSeconds = int(time.Now().Unix() - pipeline.Run.CreatedAt.Unix())
	}

	return pipelineStatus, colorCode, convertSecToDay(durationSeconds)
}

// ConvertSecToDay converts seconds passed as integer to Days, Hours, Minutes, Seconds
// Duration in D H M S format for example 124 seconds to "0D 0H 2M 4S"
func convertSecToDay(sec int) string {
	log.Debug("Duration time in seconds: ", sec)
	day := sec / (24 * 3600)

	sec = sec % (24 * 3600)
	hour := sec / 3600

	sec %= 3600
	minutes := sec / 60

	sec %= 60
	seconds := sec

	return fmt.Sprintf("%dD %dH %dM %dS", day, hour, minutes, seconds)
}

// monitorStatusAndNotify monitors for status change and
// sends notification if there is a change identified in the pipeline run status
func monitorStatusAndNotify(ctx context.Context, pipelinesMgr *pipelines.PipelinesServicesManager, branch string, pipName string, isMultiBranch bool) error {
	var previousStatus string

	retryExecutor := utils.RetryExecutor{
		Context:                  ctx,
		MaxRetries:               MaxRetries,
		RetriesIntervalMilliSecs: MinimumIntervalRetriesInMilliSecs,
		ExecutionHandler: func() (shouldRetry bool, err error) {
			pipelineStatus, err := pipelinesMgr.GetPipelineRunStatusByBranch(branch, pipName, isMultiBranch)
			if err != nil {
				// Pipelines is expected to be available. Any error is not expected and no need to retry.
				return false, err
			}
			pipeline := pipelineStatus.Pipelines[0]
			currentStatus, colorCode, duration := getPipelineStatusAndColorCode(&pipeline)
			if pipelineStatusChanged(string(currentStatus), previousStatus) {
				changedPipelineStatus := colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", PipelineName,
					pipeline.Name, Branch, pipeline.PipelineSourceBranch, Run, pipeline.Run.RunNumber, Duration,
					duration, StatusLabel, string(currentStatus))
				log.Output(changedPipelineStatus)
				if pipelineRunEnded(string(currentStatus)) {
					return false, nil
				}
			}
			previousStatus = string(currentStatus)
			// Should retry even though successful since retry mechanism is trying to fetch pipeline status continuously
			return true, nil
		},
	}

	return retryExecutor.Execute()
}

// pipelineStatusChanged returns true if the current pipeline status is different from the previous one.
// Return false otherwise.
func pipelineStatusChanged(currentStatus, previousState string) bool {
	log.Debug("Previous status: %s current status: %s", previousState, currentStatus)
	return previousState != currentStatus
}

// pipelineRunEnded if pipeline status is one of
// CANCELLED, FAILED, SUCCESS, ERROR, TIMEOUT pipeline run
// life is considered to be done.
func pipelineRunEnded(pipStatus string) bool {
	pipRunEndLife := []string{string(status.SUCCESS), string(status.FAILURE), string(status.ERROR), string(status.CANCELLED), string(status.TIMEOUT)}
	return slices.Contains(pipRunEndLife, pipStatus)
}
