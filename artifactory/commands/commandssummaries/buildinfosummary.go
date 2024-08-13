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
)

type BuildInfoSummary struct {
	platformUrl  string
	majorVersion int
}

func NewBuildInfo(platformUrl string, majorVersion int) *BuildInfoSummary {
	return &BuildInfoSummary{
		platformUrl:  platformUrl,
		majorVersion: majorVersion,
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
	tableBuilder.WriteString("\n\n ### Published Build Infos  \n\n")
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
	markdownBuilder.WriteString("\n### Modules Published As Part of This Build\n")
	var shouldGenerate bool
	for _, build := range builds {
		if modulesMarkdown := bis.generateModulesMarkdown(build.Modules...); modulesMarkdown != "" {
			markdownBuilder.WriteString(modulesMarkdown)
			shouldGenerate = true
		}
	}

	// If no supported module with artifacts was found, avoid generating the markdown.
	if !shouldGenerate {
		return ""
	}
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateModulesMarkdown(modules ...buildInfo.Module) string {
	var modulesMarkdown strings.Builder
	parentToModulesMap := groupModulesByParent(modules)
	if len(parentToModulesMap) == 0 {
		return ""
	}

	for parentModuleID, parentModules := range parentToModulesMap {
		modulesMarkdown.WriteString(fmt.Sprintf("\n#### %s\n<pre>", parentModuleID))
		shouldCollapseModuleSection := len(parentModules) > 1

		for _, module := range parentModules {
			artifactsTree := bis.createArtifactsTree(module)
			if shouldCollapseModuleSection {
				modulesMarkdown.WriteString(fmt.Sprintf("<details><summary>%s</summary>\n%s</details>", module.Id, artifactsTree))
			} else {
				modulesMarkdown.WriteString(artifactsTree)
			}
		}
		modulesMarkdown.WriteString("</pre>")
	}
	return modulesMarkdown.String()
}

func (bis *BuildInfoSummary) createArtifactsTree(module buildInfo.Module) string {
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
	return artifactsTree.String()
}

func (bis *BuildInfoSummary) generateArtifactUrl(artifact buildInfo.Artifact) string {
	if strings.TrimSpace(artifact.OriginalDeploymentRepo) == "" {
		return ""
	}
	return generateArtifactUrl(bis.platformUrl, path.Join(artifact.OriginalDeploymentRepo, artifact.Path), bis.majorVersion)
}

func groupModulesByParent(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		switch module.Type {
		case buildInfo.Docker, buildInfo.Maven, buildInfo.Npm, buildInfo.Go, buildInfo.Generic, buildInfo.Terraform:
			if len(module.Artifacts) == 0 {
				continue
			}
			if _, exists := parentToModulesMap[module.Id]; exists {
				continue
			}
			parentID := module.Parent
			if parentID == "" {
				parentID = module.Id
			}
			parentToModulesMap[parentID] = append(parentToModulesMap[parentID], module)
		default:
			continue
		}
	}
	return parentToModulesMap
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
