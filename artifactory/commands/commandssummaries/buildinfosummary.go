package commandssummaries

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
	"strings"
	"time"
)

type BuildInfoSummary struct {
	Builds []*buildInfo.BuildInfo
}

func NewBuildInfoSummary() *BuildInfoSummary {
	return &BuildInfoSummary{make([]*buildInfo.BuildInfo, 0)}
}

func (ga *BuildInfoSummary) CreateMarkdown(commandSummary any) (err error) {
	return commandsummary.CreateMarkdown(commandSummary, "build-info", ga.renderContentToMarkdown)
}

func (ga *BuildInfoSummary) renderContentToMarkdown(dataFiles []string) (markdown string, err error) {
	for _, path := range dataFiles {
		var publishBuildInfo buildInfo.BuildInfo
		if err = commandsummary.UnmarshalFromFilePath(path, &publishBuildInfo); err != nil {
			return
		}
		ga.Builds = append(ga.Builds, &publishBuildInfo)
	}

	// Generate a string that represents a Markdown table
	var markdownBuilder strings.Builder
	if len(ga.Builds) > 0 {
		if _, err = markdownBuilder.WriteString(ga.buildInfoTable()); err != nil {
			return
		}
	}
	return markdownBuilder.String(), nil

}

func (ga *BuildInfoSummary) buildInfoTable() string {
	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("\n\n| 📦 Build Info | 🕒 Timestamp | \n")
	tableBuilder.WriteString("|---------|------------| \n")
	for _, build := range ga.Builds {
		buildTime := parseBuildTime(build.Started)
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) | %s |\n", build.Name+" / "+build.Number, build.BuildUrl, buildTime))
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

func parseBuildTime(timestamp string) string {
	// Parse the timestamp string into a time.Time object
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", timestamp)
	if err != nil {
		return "N/A"
	}
	// Format the time in a more human-readable format and save it in a variable
	return t.Format("Jan 2, 2006 15:04:05")
}
