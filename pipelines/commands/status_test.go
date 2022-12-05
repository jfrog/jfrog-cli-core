package commands

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"testing"
	"time"
)

func Test_monitorStatusChange(t *testing.T) {
	type args struct {
		pipStatus string
		reStatus  string
	}
	a1 := args{"waiting", "processing"}
	a2 := args{"processing", "processing"}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"test when result status is different from previous state", a1, true},
		{"test when result status is same as previous", a2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitorStatusChange(tt.args.pipStatus, tt.args.reStatus); got != tt.want {
				t.Errorf("monitorStatusChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasPipelineRunEnded(t *testing.T) {
	type args struct {
		pipStatus string
	}
	a1 := args{"processing"}
	a2 := args{"cancelled"}
	a3 := args{"failure"}
	a4 := args{"error"}
	a5 := args{"timeout"}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"when status is not one of ending status", a1, false},
		{"when status is ending status", a2, true},
		{"when status is ending status", a3, true},
		{"when status is ending status", a4, true},
		{"when status is ending status", a5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasPipelineRunEnded(tt.args.pipStatus); got != tt.want {
				t.Errorf("hasPipelineRunEnded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_contains(t *testing.T) {
	type args struct {
		s []string
		e string
	}
	s1 := []string{"pip", "status", "trigger", "version"}
	s2 := []string{"pip", "sync", "trigger", "version"}
	s3 := []string{}
	e3 := "sync"
	e1 := "sync"
	a1 := args{s1, e1}
	a2 := args{s2, e1}
	a3 := args{s3, e3}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"should return false when s doesnt contain e ", a1, false},
		{"should return true when s contain e value", a2, true},
		{"should return false when s is empty", a3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.args.s, tt.args.e); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPipelineStatusAndColorCode(t *testing.T) {
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
	a1 := args{p}
	a2 := args{p1}
	a3 := args{p2}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 color.Color
		want2 string
	}{
		{"should return color code green", a1, "success", color.Green, "0D 0H 7M 12S"},
		{"should return color code red", a2, "failure", color.Red, "0D 0H 7M 12S"},
		{"should return color code blue", a3, "processing", color.Blue, "0D 0H 7M 12S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := getPipelineStatusAndColorCode(tt.args.pipeline)
			if got != tt.want {
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

type PipelinesMgrMock struct {
	config config.Config
	pipelines.PipelinesServicesManager
}

func (pr *PipelinesMgrMock) GetPipelineRunStatusByBranch(branch, pipName string) (*services.PipResponse, error) {
	p := services.Pipelines{ID: 1, Name: "test_build"}
	ps := []services.Pipelines{}
	ps = append(ps, p)
	s := &services.PipResponse{
		TotalCount: 1,
		Pipelines:  ps,
	}
	return s, nil
}
