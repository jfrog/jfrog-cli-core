package commands

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMonitorStatusChange(t *testing.T) {
	type args struct {
		pipeStatus string
		reStatus   string
	}
	testCases := []struct {
		name string
		args args
		want bool
	}{
		{"test when result status is different from previous state", args{"waiting", "processing"}, true},
		{"test when result status is same as previous", args{"processing", "processing"}, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, pipelineStatusChanged(testCase.args.pipeStatus, testCase.args.reStatus), testCase.want)
		})
	}
}

func TestHasPipelineRunEnded(t *testing.T) {
	type args struct {
		pipeStatus string
	}
	testCases := []struct {
		name string
		args args
		want bool
	}{
		{"when status is not one of ending status", args{"processing"}, false},
		{"when status is ending status", args{"cancelled"}, true},
		{"when status is ending status", args{"failure"}, true},
		{"when status is ending status", args{"error"}, true},
		{"when status is ending status", args{"timeout"}, true},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := pipelineRunEnded(tt.args.pipeStatus); got != tt.want {
				t.Errorf("hasPipelineRunEnded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPipelineStatusAndColorCode(t *testing.T) {
	type args struct {
		pipeline *services.Pipelines
	}
	r := services.Run{
		StatusCode:      4002,
		DurationSeconds: 432,
		CreatedAt:       time.Now(),
	}
	r1 := services.Run{
		StatusCode:      4003,
		DurationSeconds: 432,
		CreatedAt:       time.Now(),
	}
	r2 := services.Run{
		StatusCode:      4001,
		DurationSeconds: 432,
		CreatedAt:       time.Now(),
	}
	p := &services.Pipelines{
		Run: r,
	}
	p1 := &services.Pipelines{
		Run: r1,
	}
	p2 := &services.Pipelines{
		Run: r2,
	}
	testCases := []struct {
		name  string
		args  args
		want  string
		want1 color.Color
		want2 string
	}{
		{"should return color code green", args{p}, "success", color.Green, "0D 0H 7M 12S"},
		{"should return color code red", args{p1}, "failure", color.Red, "0D 0H 7M 12S"},
		{"should return color code blue", args{p2}, "processing", color.Blue, "0D 0H 7M 12S"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := getPipelineStatusAndColorCode(tt.args.pipeline)
			if string(got) != tt.want {
				t.Errorf("getPipelineStatusAndColorCode() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getPipelineStatusAndColorCode() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("getPipelineStatusAndColorCode() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestConvertSecToDay(t *testing.T) {
	type args struct {
		sec int
	}
	testCases := []struct {
		name string
		args args
		want string
	}{
		{"Test when given seconds", args{60}, "0D 0H 1M 0S"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertSecToDay(tt.args.sec); got != tt.want {
				t.Errorf("convertSecToDay() = %v, want %v", got, tt.want)
			}
		})
	}
}
