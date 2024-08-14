package commandssummaries

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path"
	"strings"
	"time"
)

const (
	timeFormat = "Jan 2, 2006 , 15:04:05"
)

type BuildInfoSummary struct {
	platformUrl     string
	majorVersion    int
	nestedFilePaths map[commandsummary.SummariesSubDirs]map[string]string
}

func NewBuildInfoWithUrl(platformUrl string, majorVersion int) *BuildInfoSummary {
	return &BuildInfoSummary{
		platformUrl:  platformUrl,
		majorVersion: majorVersion,
	}
}
func NewBuildInfo() *BuildInfoSummary {
	return &BuildInfoSummary{}
}

func (bis *BuildInfoSummary) GenerateMarkdownFromFiles(dataFilePaths []string, nestedFilePaths map[commandsummary.SummariesSubDirs]map[string]string) (finalMarkdown string, err error) {
	bis.nestedFilePaths = nestedFilePaths
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
	tableBuilder.WriteString("\n\n### Published Build Info\n\n")
	tableBuilder.WriteString("\n\n|  Build Info |  Time Stamp | Scan Result \n")
	tableBuilder.WriteString("|---------|------------|------------| \n")
	for _, build := range builds {
		buildTime := parseBuildTime(build.Started)
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) | %s | %s |\n", build.Name+" "+build.Number, build.BuildUrl, buildTime, bis.getScanResultsMarkdown(build)))
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

func (bis *BuildInfoSummary) getScanResultsMarkdown(build *buildInfo.BuildInfo) (nestedMarkdown []byte) {
	nestedMarkdown = []byte("<pre>🚨 Artifact was not scanned in the job!</pre>")
	var scanResult string
	scanResult, ok := bis.nestedFilePaths["build-scan"][build.Name+"-"+build.Number]
	if !ok {
		return
	}
	nestedMarkdown, err := fileutils.ReadFile(scanResult)
	if err != nil {
		log.Warn("failed to read build scan results for build: " + build.Name + "-" + build.Number)
		return
	}
	// Replace new lines with <br> to preserve the formatting in the markdown table
	nestedMarkdown = []byte(strings.ReplaceAll(string(nestedMarkdown), "\n", "<br>"))
	return
}

func (bis *BuildInfoSummary) buildInfoModules(builds []*buildInfo.BuildInfo) string {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString("\n\n### Published Modules\n\n")
	var shouldGenerate bool
	for _, build := range builds {
		for _, module := range build.Modules {
			if len(module.Artifacts) == 0 {
				continue
			}
			switch module.Type {
			case buildInfo.Docker, buildInfo.Maven, buildInfo.Npm, buildInfo.Go, buildInfo.Generic, buildInfo.Terraform:
				markdownBuilder.WriteString(bis.generateModuleMarkdown(module, bis.getScanResultsMarkdown(build)))
				shouldGenerate = true
			default:
				// Skip unsupported module types.
				continue
			}
		}
	}

	// If no supported module with artifacts was found, avoid generating the markdown.
	if !shouldGenerate {
		return ""
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

func (bis *BuildInfoSummary) generateModuleMarkdown(module buildInfo.Module, securityMarkdown []byte) string {
	var moduleMarkdown strings.Builder
	moduleMarkdown.WriteString(fmt.Sprintf("\n#### `%s`\n", module.Id))
	moduleMarkdown.WriteString("\n\n|      Artifacts   |     Security Issues     | \n")
	moduleMarkdown.WriteString("|-----------------------|---------------------------------------| \n")

	artifactsTree := utils.NewFileTree()
	for _, artifact := range module.Artifacts {
		artifactUrlInArtifactory := bis.generateArtifactUrl(artifact)
		if artifact.OriginalDeploymentRepo == "" {
			// Placeholder needed to build an artifact tree when repo is missing.
			artifact.OriginalDeploymentRepo = " "
		}
		artifactTreePath := path.Join(artifact.OriginalDeploymentRepo, artifact.Path)
		artifactsTree.AddFile(artifactTreePath, artifactUrlInArtifactory)
	}
	content := strings.ReplaceAll(artifactsTree.String(), "\n", "<br>") + "|" + string(securityMarkdown)
	moduleMarkdown.WriteString("| <pre>" + content + "</pre>")
	return moduleMarkdown.String()
}

func (bis *BuildInfoSummary) generateArtifactUrl(artifact buildInfo.Artifact) string {
	if strings.TrimSpace(artifact.OriginalDeploymentRepo) == "" {
		return ""
	}
	return generateArtifactUrl(bis.platformUrl, path.Join(artifact.OriginalDeploymentRepo, artifact.Path), bis.majorVersion)
}
