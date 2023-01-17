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
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"time"
)

type StatusCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	pipelineName  string
	notify        bool
	isMultiBranch bool
}

const (
	PipelineName = "PipelineName :"
	Branch       = "Branch :"
	Run          = "Run :"
	Duration     = "Duration :"
	StatusLabel  = "Status :"
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
	return "status"
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
	serviceManager, svcMgrErr := manager.CreateServiceManager(sc.serverDetails)
	if svcMgrErr != nil {
		return svcMgrErr
	}

	// Get pipeline status using branch name, pipelines name and whether it is multi branch
	matchingPipes, pipeStatusErr := serviceManager.GetPipelineRunStatusByBranch(sc.branch, sc.pipelineName, sc.isMultiBranch)
	if pipeStatusErr != nil {
		return pipeStatusErr
	}
	var res string
	for i := range matchingPipes.Pipelines {
		pipe := matchingPipes.Pipelines[i]
		// Filter out pipelines which are not run at all
		if pipe.LatestRunID != 0 {
			// When notification option is selected use this flow to notify
			if sc.pipelineName != "" && sc.notify {
				monErr := monitorStatusAndNotify(context.Background(), serviceManager, sc.branch, sc.pipelineName, sc.isMultiBranch)
				if monErr != nil {
					return monErr
				}
			} else {
				respStatus, colorCode, duration := getPipelineStatusAndColorCode(&pipe)
				res = res + colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", "PipelineName:", pipe.Name, "Branch:", pipe.PipelineSourceBranch, "Run:", pipe.Run.RunNumber, "Duration:", duration, "Status:", respStatus)
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

// monitorStatusAndNotify monitor for status change and
// Send notification if there is a change identified in the pipeline run status
func monitorStatusAndNotify(ctx context.Context, pipelinesMgr *pipelines.PipelinesServicesManager, branch string, pipName string, isMultiBranch bool) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	var reStatus string
	for {
		select {
		case <-ctx.Done():
			return errorutils.CheckError(ctx.Err())
		default:
			pipelineStatus, err := pipelinesMgr.GetPipelineRunStatusByBranch(branch, pipName, isMultiBranch)
			if err != nil {
				return err
			}
			pipeline := pipelineStatus.Pipelines[0]
			statusValue, colorCode, duration := getPipelineStatusAndColorCode(&pipeline)
			if monitorStatusChange(string(statusValue), reStatus) {
				res := colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", PipelineName,
					pipeline.Name, Branch, pipeline.PipelineSourceBranch, Run, pipeline.Run.RunNumber, Duration,
					duration, StatusLabel, string(statusValue))
				log.Info(res)
				if hasPipelineRunEnded(string(statusValue)) {
					return nil
				}
			}
			reStatus = string(statusValue)
			time.Sleep(5 * time.Second)
		}
	}
}

// Check for change in status with the previous status
// Return true if there is a change in status with previous status
// Return false if current and previous status is same
func monitorStatusChange(pipStatus, reStatus string) bool {
	if reStatus == pipStatus {
		return false
	}
	reStatus = pipStatus
	log.Debug("Previous status: %s current status: %s", reStatus, pipStatus)
	return true
}

// HasPipelineRunEnded if pipeline status is one of
// CANCELLED, FAILED, SUCCESS, ERROR, TIMEOUT pipeline run
// life is considered to be done.
func hasPipelineRunEnded(pipStatus string) bool {
	pipRunEndLife := []string{string(status.SUCCESS), string(status.FAILURE), string(status.ERROR), string(status.CANCELLED), string(status.TIMEOUT)}
	return slices.Contains(pipRunEndLife, pipStatus)
}
