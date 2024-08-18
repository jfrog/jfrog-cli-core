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
	platformUrl          string
	platformMajorVersion int
}

func NewBuildInfoSummary(serverUrl string, platformMajorVersion int) (*commandsummary.CommandSummary, error) {
	return commandsummary.New(&BuildInfoSummary{
		platformUrl:          serverUrl,
		platformMajorVersion: platformMajorVersion,
	}, "build-info")
}

func (bis *BuildInfoSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	// Aggregate all the build info files into a slice
	var builds []*buildInfo.BuildInfo
	for _, filePath := range dataFilePaths {
		var publishBuildInfo buildInfo.BuildInfo
		if err = commandsummary.UnmarshalFromFilePath(filePath, &publishBuildInfo); err != nil {
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
	tableBuilder.WriteString("\n\n### Published Build Infos\n\n")
	tableBuilder.WriteString("\n\n|  Build Info |  Time Stamp |\n")
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
	markdownBuilder.WriteString("\n\n### Modules Published As Part of This Build\n\n")
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
		modulesMarkdown.WriteString(fmt.Sprintf("#### %s\n<pre>", parentModuleID))
		isMultiModule := len(parentModules) > 1

		for _, module := range parentModules {
			if isMultiModule && parentModuleID == module.Id {
				// Skip the parent module if there are multiple modules, as it will be displayed as a header
				continue
			}
			artifactsTree := bis.createArtifactsTree(module)
			if isMultiModule {
				// Collapse the module tree if there are multiple modules
				modulesMarkdown.WriteString(fmt.Sprintf("<details><summary>%s</summary>\n%s</details>", module.Id, artifactsTree))
			} else {
				modulesMarkdown.WriteString(artifactsTree)
			}
		}
		modulesMarkdown.WriteString("</pre>\n")
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
	return generateArtifactUrl(bis.platformUrl, path.Join(artifact.OriginalDeploymentRepo, artifact.Path), bis.platformMajorVersion)
}

// groupModulesByParent groups modules that share the same parent ID into a map where the key is the parent ID and the value is a slice of those modules.
func groupModulesByParent(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		if len(module.Artifacts) == 0 || !isSupportedModuleType(module.Type) {
			continue
		}

		parentID := module.Parent
		// If the module has no parent, that means it is the parent module itself, so we can use its ID as the parent ID.
		if parentID == "" {
			parentID = module.Id
		}
		parentToModulesMap[parentID] = append(parentToModulesMap[parentID], module)
	}
	return parentToModulesMap
}

func isSupportedModuleType(moduleType buildInfo.ModuleType) bool {
	switch moduleType {
	case buildInfo.Docker, buildInfo.Maven, buildInfo.Npm, buildInfo.Go, buildInfo.Generic, buildInfo.Terraform:
		return true
	default:
		return false
	}
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
