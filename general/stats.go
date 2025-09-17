package general

import (
	"github.com/jfrog/jfrog-cli-artifactory/stats"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const displayLimit = 5

type Stats struct {
	Product      string
	FilterName   string
	FormatOutput string
	AccessToken  string
	ServerId     string
	DisplayLimit int
}

func NewStatsCommand() *Stats {
	return &Stats{DisplayLimit: displayLimit}
}

func (s *Stats) SetFilterName(name string) *Stats {
	s.FilterName = name
	return s
}

func (s *Stats) SetFormatOutput(format string) *Stats {
	s.FormatOutput = format
	return s
}

func (s *Stats) SetAccessToken(token string) *Stats {
	s.AccessToken = token
	return s
}

func (s *Stats) SetServerId(id string) *Stats {
	s.ServerId = id
	return s
}

func (s *Stats) SetProduct(product string) *Stats {
	s.Product = product
	return s
}

func (ss *Stats) Run() error {
	switch ss.Product {
	case "rt", "artifactory", "artifactories":
		newStatsCommand := ss.NewArtifactoryStatsCommand()
		return newStatsCommand.Run()
	default:
		log.Error("Unknown product: " + ss.Product)
		return nil
	}
}

func (ss *Stats) NewArtifactoryStatsCommand() *stats.StatsArtifactory {
	newStatsCommand := stats.NewArtifactoryStatsCommand().
		SetServerId(ss.ServerId).SetAccessToken(ss.AccessToken).
		SetFilterName(ss.FilterName).SetFormatOutput(ss.FormatOutput).
		SetDisplayLimit(ss.DisplayLimit)
	return newStatsCommand
}
