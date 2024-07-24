package commandssummaries

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
	"path"
	"strings"
	"time"
)

const (
	timeFormat = "Jan 2, 2006 , 15:04:05"
	UiFormat   = "%sui/repos/tree/General/%s/%s"
)

type BuildInfoSummary struct {
	// Used to generate artifact links
	serverUrl string
}

func NewBuildInfo(serverUrl string) *BuildInfoSummary {
	return &BuildInfoSummary{
		serverUrl: serverUrl,
	}
}

func (bis *BuildInfoSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	// Aggregate all the build info files into a slice
	var builds []*buildInfo.BuildInfo
	for _, path := range dataFilePaths {
		var publishBuildInfo buildInfo.BuildInfo
		if err = commandsummary.UnmarshalFromFilePath(path, &publishBuildInfo); err != nil {
			return
		}
		builds = append(builds, &publishBuildInfo)
	}

	if len(builds) > 0 {
		finalMarkdown = bis.buildInfoTable(builds) + bis.buildInfoModules(builds)
	}
	return
}

func (bis *BuildInfoSummary) buildInfoTable(builds []*buildInfo.BuildInfo) string {
	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("\n\n ### Published Builds  \n\n")
	tableBuilder.WriteString("\n\n|  Build Info |  Time Stamp | \n")
	tableBuilder.WriteString("|---------|------------| \n")
	for _, build := range builds {
		buildTime := parseBuildTime(build.Started)
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) | %s |\n", build.Name+" "+build.Number, build.BuildUrl, buildTime))
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

func (bis *BuildInfoSummary) buildInfoModules(builds []*buildInfo.BuildInfo) string {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString("\n\n ### Published Modules  \n\n")
	for _, build := range builds {
		for _, module := range build.Modules {
			if module.Type == "generic" {
				continue // Skip generic modules to avoid overlaps
			}
			markdownBuilder.WriteString(bis.generateModuleMarkdown(module))
		}
	}
	return markdownBuilder.String()
}

func parseBuildTime(timestamp string) string {
	// Parse the timestamp string into a time.Time object
	buildInfoTime, err := time.Parse(buildInfo.TimeFormat, timestamp)
	if err != nil {
		return "N/A"
	}
	// Format the time in a more human-readable format and save it in a variable
	return buildInfoTime.Format(timeFormat)
}

func (bis *BuildInfoSummary) generateModuleMarkdown(module buildInfo.Module) string {
	var moduleMarkdown strings.Builder
	moduleMarkdown.WriteString(fmt.Sprintf("\n ### `%s` \n", module.Id))
	artifactsTree := utils.NewFileTree()
	for _, artifact := range module.Artifacts {
		artifactUrlInArtifactory := bis.generateArtifactUrl(artifact)
		if artifact.OriginalRepo == "" {
			// Placeholder to show in the tree that the artifact is not in a repo
			artifact.OriginalRepo = " "
		}
		artifactTreePath := path.Join(artifact.OriginalRepo, module.Id, artifact.Name)
		artifactsTree.AddFile(artifactTreePath, artifactUrlInArtifactory)
	}
	moduleMarkdown.WriteString("\n\n <pre>" + artifactsTree.String() + "</pre>")
	return moduleMarkdown.String()
}

func (bis *BuildInfoSummary) generateArtifactUrl(artifact buildInfo.Artifact) string {
	if artifact.OriginalRepo != "" {
		return fmt.Sprintf(UiFormat, bis.serverUrl, artifact.OriginalRepo, artifact.Path)
	}
	return ""
}
