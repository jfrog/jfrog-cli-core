package status

import (
	"github.com/gookit/color"
	"testing"
)

func TestGetStatusColorCode(t *testing.T) {
	type args struct {
		status PipelineStatus
	}
	testCases := []struct {
		name string
		args args
		want color.Color
	}{
		{"get color code when status is success", args{SUCCESS}, color.Green},
		{"get color code when status is failure", args{FAILURE}, color.Red},
		{"get color code when status is processing", args{PROCESSING}, color.Blue},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := GetStatusColorCode(testCase.args.status); got != testCase.want {
				t.Errorf("GetStatusColorCode() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestGetPipelineStatus(t *testing.T) {
	type args struct {
		statusCode int
	}
	testCases := []struct {
		name string
		args args
		want string
	}{
		{"should return queued for status code 4000", args{4000}, "queued"},
		{"should return processing for status code 4001", args{4001}, "processing"},
		{"should return success for status code 4002", args{4002}, "success"},
		{"should return failure for status code 4003", args{4003}, "failure"},
		{"should return error for status code 4004", args{4004}, "error"},
		{"should return waiting for status code 4005", args{4005}, "waiting"},
		{"should return cancelled for status code 4006", args{4006}, "cancelled"},
		{"should return unstable for status code 4007", args{4007}, "unstable"},
		{"should return skipped for status code 4008", args{4008}, "skipped"},
		{"should return timeout for status code 4009", args{4009}, "timeout"},
		{"should return stopped for status code 4010", args{4010}, "stopped"},
		{"should return deleted for status code 4011", args{4011}, "deleted"},
		{"should return cached for status code 4012", args{4012}, "cached"},
		{"should return cancelling for status code 4013", args{4013}, "cancelling"},
		{"should return timingOut for status code 4014", args{4014}, "timingOut"},
		{"should return creating for status code 4015", args{4015}, "creating"},
		{"should return ready for status code 4016", args{4016}, "ready"},
		{"should return online for status code 4017", args{4017}, "online"},
		{"should return offline for status code 4018", args{4018}, "offline"},
		{"should return unhealthy for status code 4019", args{4019}, "unhealthy"},
		{"should return onlineRequested for status code 4020", args{4020}, "onlineRequested"},
		{"should return offlineRequested for status code 4021", args{4021}, "offlineRequested"},
		{"should return pendingApproval for status code 4022", args{4022}, "pendingApproval"},
		{"should return un defined for status code other than in range [4000 - 4022]", args{9999}, "notDefined"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := GetPipelineStatus(testCase.args.statusCode); string(got) != testCase.want {
				t.Errorf("GetPipelineStatus() = %v, want %v", got, testCase.want)
			}
		})
	}
}
