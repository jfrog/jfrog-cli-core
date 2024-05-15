package jobsummariesimpl

import (
	"encoding/json"
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/jobsummaries"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
	"time"
)

type GithubSummaryBpImpl struct {
	Builds []*buildInfo.BuildInfo
}

func (ga *GithubSummaryBpImpl) CreateSummaryMarkdown(content any, section jobsummaries.MarkdownSection) (err error) {
	return jobsummaries.CreatSummaryMarkdownBaseImpl(content, section, ga.appendResultObject, ga.renderContentToMarkdown)
}

// Implement this function to accept an object you'd like to save into the file system as an array form of the object to allow aggregation
func (ga *GithubSummaryBpImpl) appendResultObject(currentResult interface{}, previousResults []byte) ([]byte, error) {
	build, ok := currentResult.(*buildInfo.BuildInfo)
	if !ok {
		return nil, fmt.Errorf("failed to cast currentResult to buildInfo.BuildInfo")
	}
	// Unmarshal the data into an array of build info objects
	var builds []*buildInfo.BuildInfo
	if len(previousResults) > 0 {
		err := json.Unmarshal(previousResults, &builds)
		if err != nil {
			return nil, err
		}
	}
	// Append the new build info object to the array
	builds = append(builds, build)
	return json.Marshal(builds)
}

func (ga *GithubSummaryBpImpl) renderContentToMarkdown(content []byte) (markdown string, err error) {
	// Unmarshal the data into an array of build info objects
	if err = json.Unmarshal(content, &ga.Builds); err != nil {
		log.Error("Failed to unmarshal data: ", err)
		return
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

func (ga *GithubSummaryBpImpl) buildInfoTable() string {
	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("\n\n| 🔢 Build Info | 🕒 Timestamp | \n")
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
