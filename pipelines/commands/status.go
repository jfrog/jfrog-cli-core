package commands

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"time"
)

type StatusCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	pipelineName  string
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

func (sc *StatusCommand) Run() (string, error) {
	var err error
	serviceManager, err := manager.CreateServiceManager(sc.serverDetails)
	if err != nil {
		return "", err
	}
	pipelines, err := serviceManager.GetPipelineRunStatusByBranch(sc.branch, sc.pipelineName)
	if err != nil {
		return "", err
	}
	var res string
	for i := range pipelines.Pipelines {
		p := pipelines.Pipelines[i]
		reStatus, colorCode, d := getPipelineStatusAndColorCode(&p)
		res = res + colorCode.Sprintf("\n%s %s\n%14s %s\n%14s %d \n%14s %s \n%14s %s %s\n", "PipelineName :", p.Name, "Branch :", p.PipelineSourceBranch, "Run :", p.Run.RunNumber, "Duration :", d, "Status :", reStatus)
	}
	return res, nil
}

/*
 * getPipelineStatusAndColorCode based on pipeline status code
 * return color to be used for pretty printing
 */
func getPipelineStatusAndColorCode(pipeline *services.Pipelines) (string, color.Color, string) {
	s := status.GetPipelineStatus(pipeline.Run.StatusCode)
	colorCode := status.GetStatusColorCode(s)
	d := pipeline.Run.DurationSeconds
	if d == 0 {
		t1 := pipeline.Run.StartedAt
		t2 := time.Now()
		d = int(t2.Sub(t1))
	}

	t := ConvertSecToDay(d)
	return s, colorCode, t
}

/*
ConvertSecToDay converts seconds passed as integer to
- D H M S format
*/
func ConvertSecToDay(sec int) string {
	day := sec / (24 * 3600)

	sec = sec % (24 * 3600)
	hour := sec / 3600

	sec %= 3600
	minutes := sec / 60

	sec %= 60
	seconds := sec

	v := fmt.Sprintf("%dD %dH %dM %dS", day, hour, minutes, seconds)
	return v
}
