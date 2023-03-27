package status

import "github.com/gookit/color"

type PipelineStatus string

const (
	QUEUED           PipelineStatus = "queued"
	PROCESSING       PipelineStatus = "processing"
	SUCCESS          PipelineStatus = "success"
	FAILURE          PipelineStatus = "failure"
	ERROR            PipelineStatus = "error"
	CANCELLED        PipelineStatus = "cancelled"
	TIMEOUT          PipelineStatus = "timeout"
	WAITING          PipelineStatus = "waiting"
	SKIPPED          PipelineStatus = "skipped"
	UNSTABLE         PipelineStatus = "unstable"
	STOPPED          PipelineStatus = "stopped"
	DELETED          PipelineStatus = "deleted"
	CACHED           PipelineStatus = "cached"
	CANCELLING       PipelineStatus = "cancelling"
	TIMINGOUT        PipelineStatus = "timingOut"
	CREATING         PipelineStatus = "creating"
	READY            PipelineStatus = "ready"
	ONLINE           PipelineStatus = "online"
	OFFLINE          PipelineStatus = "offline"
	UNHEALTHY        PipelineStatus = "unhealthy"
	ONLINEREQUESTED  PipelineStatus = "onlineRequested"
	OFFLINEREQUESTED PipelineStatus = "offlineRequested"
	PENDINGAPPROVAL  PipelineStatus = "pendingApproval"
	NOTDEFINED       PipelineStatus = "notDefined"
)

// GetStatusColorCode returns gokit/color.Color
// based on status input parameter
func GetStatusColorCode(status PipelineStatus) color.Color {
	switch status {
	case SUCCESS:
		return color.Green
	case FAILURE, ERROR, CANCELLED, TIMEOUT:
		return color.Red
	}
	return color.Blue
}

// GetPipelineStatus based on pipelines reStatus code
// returns respective reStatus in string format
// for eq:- 4002 return success
func GetPipelineStatus(statusCode int) PipelineStatus {
	switch statusCode {
	case 4000:
		return QUEUED
	case 4001:
		return PROCESSING
	case 4002:
		return SUCCESS
	case 4003:
		return FAILURE
	case 4004:
		return ERROR
	case 4005:
		return WAITING
	case 4006:
		return CANCELLED
	case 4007:
		return UNSTABLE
	case 4008:
		return SKIPPED
	case 4009:
		return TIMEOUT
	case 4010:
		return STOPPED
	case 4011:
		return DELETED
	case 4012:
		return CACHED
	case 4013:
		return CANCELLING
	case 4014:
		return TIMINGOUT
	case 4015:
		return CREATING
	case 4016:
		return READY
	case 4017:
		return ONLINE
	case 4018:
		return OFFLINE
	case 4019:
		return UNHEALTHY
	case 4020:
		return ONLINEREQUESTED
	case 4021:
		return OFFLINEREQUESTED
	case 4022:
		return PENDINGAPPROVAL
	}
	return NOTDEFINED
}

// GetRunCompletedStatusList returns a string slice with status values
// which can be considered as run of the step/pipeline is completed
// and there is nothing left to continue on the step/pipelines
func GetRunCompletedStatusList() []PipelineStatus {
	return []PipelineStatus{SUCCESS, FAILURE, ERROR, CANCELLED, SKIPPED}
}

// GetWaitingForRunAndRunningSteps returns a string slice with staus values
// which define a step/pipeline that work is yet to be completed
func GetWaitingForRunAndRunningSteps() []PipelineStatus {
	return []PipelineStatus{PROCESSING, WAITING, "creating", "ready", QUEUED}
}
