package commands

import (
	"context"
	"fmt"
	"github.com/gen2brain/beeep"
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

type StatusCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	pipelineName  string
	notify        bool
	isMultiBranch bool
}

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
	// create service manager to fetch run status
	serviceManager, svcMgrErr := manager.CreateServiceManager(sc.serverDetails)
	if svcMgrErr != nil {
		return "", svcMgrErr
	}

	// get pipeline status using branch name, pipelines name and whether it is multi branch
	matchingPipes, pipStatusErr := serviceManager.GetPipelineRunStatusByBranch(sc.branch, sc.pipelineName, sc.isMultiBranch)
	if pipStatusErr != nil {
		return "", errorutils.CheckError(pipStatusErr)
	}
	var res string
	for i := range matchingPipes.Pipelines {
		p := matchingPipes.Pipelines[i]
		if p.LatestRunID != 0 { // filter out pipelines which are not run at all
			if sc.pipelineName != "" && sc.notify { // when notification option is selected use this flow to notify
				monErr := monitorStatusAndNotify(context.Background(), serviceManager, sc.branch, sc.pipelineName, sc.isMultiBranch)
				if monErr != nil {
					return "", errorutils.CheckError(monErr)
				}
			} else {
				reStatus, colorCode, d := getPipelineStatusAndColorCode(&p)
				res := colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", "PipelineName :", p.Name, "Branch :", p.PipelineSourceBranch, "Run :", p.Run.RunNumber, "Duration :", d, "Status :", reStatus)
				return res, nil
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
	d := pipeline.Run.DurationSeconds
	if d == 0 {
		// calculate duration it took till now by differentiating created time with current time
		t1 := pipeline.Run.CreatedAt
		t2 := time.Now()
		d = int(t2.Unix() - t1.Unix())
	}

	return s, colorCode, convertSecToDay(d)
}

// ConvertSecToDay converts seconds passed as integer to
// - D H M S format
func convertSecToDay(sec int) string {
	log.Debug("received duration is: ", sec)
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
// send notification if there is a change identified
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
				return errorutils.CheckError(err)
			}
			pipeline := p.Pipelines[0]
			s, colorCode, d := getPipelineStatusAndColorCode(&pipeline)
			if monitorStatusChange(s, reStatus) {
				res := colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s\n", "PipelineName :", pipeline.Name, "Branch :", pipeline.PipelineSourceBranch, "Run :", pipeline.Run.RunNumber, "Duration :", d, "Status :", reStatus)
				log.Info(res)
				sendNotification(s, pipeline.Name)
				if hasPipelineRunEnded(s) {
					return nil
				}
			}
			reStatus = s
			time.Sleep(5 * time.Second)
		}
	}
}

// check for change in status with the latest status
func monitorStatusChange(pipStatus, reStatus string) bool {
	if reStatus == pipStatus {
		return false
	}
	reStatus = pipStatus
	log.Debug("previous status : %s current status: %s", reStatus, pipStatus)
	return true
}

// hasPipelineRunEnded if pipeline status is one of
// CANCELLED, FAILED, SUCCESS, ERROR, TIMEOUT pipeline run
// life is considered to be done.
func hasPipelineRunEnded(pipStatus string) bool {
	pipRunEndLife := []string{status.SUCCESS, status.FAILURE, status.ERROR, status.CANCELLED, status.TIMEOUT}
	return contains(pipRunEndLife, pipStatus)
}

// sendNotification sends notification
func sendNotification(pipStatus string, pipName string) {
	err := beeep.Alert("Pipelines CLI", pipName+" - "+pipStatus, "")
	if err != nil {
		log.Warn("failed to send notification")
	}
}

// contains returns whether a string is available in slice
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
