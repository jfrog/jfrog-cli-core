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

func (sc *StatusCommand) Run() (string, error) {
	// Create service manager to fetch run status
	serviceManager, svcMgrErr := manager.CreateServiceManager(sc.serverDetails)
	if svcMgrErr != nil {
		return "", svcMgrErr
	}

	// Get pipeline status using branch name, pipelines name and whether it is multi branch
	matchingPipes, pipeStatusErr := serviceManager.GetPipelineRunStatusByBranch(sc.branch, sc.pipelineName, sc.isMultiBranch)
	if pipeStatusErr != nil {
		return "", pipeStatusErr
	}
	var res string
	for i := range matchingPipes.Pipelines {
		pipe := matchingPipes.Pipelines[i]
		if pipe.LatestRunID != 0 { // Filter out pipelines which are not run at all
			if sc.pipelineName != "" && sc.notify { // When notification option is selected use this flow to notify
				monErr := monitorStatusAndNotify(context.Background(), serviceManager, sc.branch, sc.pipelineName, sc.isMultiBranch)
				if monErr != nil {
					return "", monErr
				}
			} else {
				respStatus, colorCode, duration := getPipelineStatusAndColorCode(&pipe)
				return colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", "PipelineName :", pipe.Name, "Branch :", pipe.PipelineSourceBranch, "Run :", pipe.Run.RunNumber, "Duration :", duration, "Status :", respStatus), nil
			}
		}
	}
	return res, nil
}

// getPipelineStatusAndColorCode based on pipeline status code
// return color to be used for pretty printing
func getPipelineStatusAndColorCode(pipeline *services.Pipelines) (string, color.Color, string) {
	s := status.GetPipelineStatus(pipeline.Run.StatusCode)
	colorCode := status.GetStatusColorCode(s)
	durationSeconds := pipeline.Run.DurationSeconds
	if durationSeconds == 0 {
		// Calculate the duration time by differentiating created time from the current time
		durationSeconds = int(time.Now().Unix() - pipeline.Run.CreatedAt.Unix())
	}

	return s, colorCode, convertSecToDay(durationSeconds)
}

// ConvertSecToDay converts seconds passed as integer to
// - D H M S format
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
// Send notification if there is a change identified
func monitorStatusAndNotify(ctx context.Context, pipelinesMgr *pipelines.PipelinesServicesManager, branch string, pipName string, isMultiBranch bool) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	var reStatus string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p, err := pipelinesMgr.GetPipelineRunStatusByBranch(branch, pipName, isMultiBranch)
			if err != nil {
				return err
			}
			pipeline := p.Pipelines[0]
			statusValue, colorCode, duration := getPipelineStatusAndColorCode(&pipeline)
			if monitorStatusChange(statusValue, reStatus) {
				res := colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", PipelineName,
					pipeline.Name, Branch, pipeline.PipelineSourceBranch, Run, pipeline.Run.RunNumber, Duration,
					duration, StatusLabel, reStatus)
				log.Info(res)
				if hasPipelineRunEnded(statusValue) {
					return nil
				}
			}
			reStatus = statusValue
			time.Sleep(5 * time.Second)
		}
	}
}

// Check for change in status with the latest status
func monitorStatusChange(pipStatus, reStatus string) bool {
	if reStatus == pipStatus {
		return false
	}
	reStatus = pipStatus
	log.Debug("Previous status : %s current status: %s", reStatus, pipStatus)
	return true
}

// HasPipelineRunEnded if pipeline status is one of
// CANCELLED, FAILED, SUCCESS, ERROR, TIMEOUT pipeline run
// life is considered to be done.
func hasPipelineRunEnded(pipStatus string) bool {
	pipRunEndLife := []string{status.SUCCESS, status.FAILURE, status.ERROR, status.CANCELLED, status.TIMEOUT}
	return slices.Contains(pipRunEndLife, pipStatus)
}
