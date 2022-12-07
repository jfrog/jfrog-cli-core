package status

import (
	"github.com/gookit/color"
	"testing"
)

func TestGetStatusColorCode(t *testing.T) {
	type args struct {
		status string
	}
	a1 := args{"success"}
	a2 := args{"failure"}
	a3 := args{"processing"}
	tests := []struct {
		name string
		args args
		want color.Color
	}{
		{"get color code when status is success", a1, color.Green},
		{"get color code when status is failure", a2, color.Red},
		{"get color code when status is processing", a3, color.Blue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetStatusColorCode(tt.args.status); got != tt.want {
				t.Errorf("GetStatusColorCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPipelineStatus(t *testing.T) {
	type args struct {
		statusCode int
	}
	a1 := args{4000}
	a2 := args{4001}
	a3 := args{4002}
	a4 := args{4003}
	a5 := args{4004}
	a6 := args{4005}
	a7 := args{4006}
	a8 := args{4007}
	a9 := args{4008}
	a10 := args{4009}
	a11 := args{4010}
	a12 := args{4011}
	a13 := args{4012}
	a14 := args{4013}
	a15 := args{4014}
	a16 := args{4015}
	a17 := args{4016}
	a18 := args{4017}
	a19 := args{4018}
	a20 := args{4019}
	a21 := args{4020}
	a22 := args{4021}
	a23 := args{4022}
	a24 := args{9999}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"should return queued for status code 4000", a1, "queued"},
		{"should return processing for status code 4001", a2, "processing"},
		{"should return success for status code 4002", a3, "success"},
		{"should return failure for status code 4003", a4, "failure"},
		{"should return error for status code 4004", a5, "error"},
		{"should return waiting for status code 4005", a6, "waiting"},
		{"should return cancelled for status code 4006", a7, "cancelled"},
		{"should return unstable for status code 4007", a8, "unstable"},
		{"should return skipped for status code 4008", a9, "skipped"},
		{"should return timeout for status code 4009", a10, "timeout"},
		{"should return stopped for status code 4010", a11, "stopped"},
		{"should return deleted for status code 4011", a12, "deleted"},
		{"should return cached for status code 4012", a13, "cached"},
		{"should return cancelling for status code 4013", a14, "cancelling"},
		{"should return timingOut for status code 4014", a15, "timingOut"},
		{"should return creating for status code 4015", a16, "creating"},
		{"should return ready for status code 4016", a17, "ready"},
		{"should return online for status code 4017", a18, "online"},
		{"should return offline for status code 4018", a19, "offline"},
		{"should return unhealthy for status code 4019", a20, "unhealthy"},
		{"should return onlineRequested for status code 4020", a21, "onlineRequested"},
		{"should return offlineRequested for status code 4021", a22, "offlineRequested"},
		{"should return pendingApproval for status code 4022", a23, "pendingApproval"},
		{"should return un defined for status code other than in range [4000 - 4022]", a24, "NOT DEFINED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPipelineStatus(tt.args.statusCode); got != tt.want {
				t.Errorf("GetPipelineStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
