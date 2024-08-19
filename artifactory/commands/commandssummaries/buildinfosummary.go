package commandssummaries

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
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
		modulesMarkdown.WriteString(fmt.Sprintf("#### %s\n<pre>", parentModuleID))
		isMultiModule := len(parentModules) > 1

		for _, module := range parentModules {
			if isMultiModule && parentModuleID == module.Id {
				// Skip the parent module if there are multiple modules, as it will be displayed as a header
				continue
			}
			modulesMarkdown.WriteString(bis.generateModuleArtifactsTree(&module, isMultiModule))
		}
		modulesMarkdown.WriteString("</pre>\n")
	}
	return modulesMarkdown.String()
}

func (bis *BuildInfoSummary) generateModuleArtifactsTree(module *buildInfo.Module, shouldCollapseArtifactsTree bool) string {
	artifactsTree := bis.createArtifactsTree(module)
	if shouldCollapseArtifactsTree {
		return bis.generateModuleCollapsibleSection(module, artifactsTree)
	}
	return artifactsTree
}

func (bis *BuildInfoSummary) generateModuleCollapsibleSection(module *buildInfo.Module, sectionContent string) string {
	switch module.Type {
	case buildInfo.Docker:
		return createCollapsibleSection(createDockerMultiArchTitle(module, bis.platformUrl), sectionContent)
	default:
		return createCollapsibleSection(module.Id, sectionContent)
	}
}

func (bis *BuildInfoSummary) createArtifactsTree(module *buildInfo.Module) string {
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

// groupModulesByParent groups modules that share the same parent ID into a map where the key is the parent ID and the value is a slice of those modules.
func groupModulesByParent(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		if len(module.Artifacts) == 0 || !isSupportedModule(&module) {
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

func isSupportedModule(module *buildInfo.Module) bool {
	switch module.Type {
	case buildInfo.Maven, buildInfo.Npm, buildInfo.Go, buildInfo.Generic, buildInfo.Terraform:
		return true
	case buildInfo.Docker:
		// Skip attestations that are added as a module for multi-arch docker builds
		return !strings.HasPrefix(module.Id, container.AttestationsModuleIdPrefix)
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

func createDockerMultiArchTitle(module *buildInfo.Module, platformUrl string) string {
	// Extract the parent image name from the module ID (e.g. my-image:1.0 -> my-image)
	parentImageName := strings.Split(module.Parent, ":")[0]

	// Get the relevant SHA256
	var sha256 string
	for _, artifact := range module.Artifacts {
		if artifact.Name == container.ManifestJsonFile {
			sha256 = artifact.Sha256
			break
		}
	}
	// Create a link to the Docker package in Artifactory UI
	dockerModuleLink := fmt.Sprintf(artifactoryDockerPackagesUiFormat, strings.TrimSuffix(platformUrl, "/"), "%2F%2F"+parentImageName, sha256)
	return fmt.Sprintf("%s <a href=%s>(🐸 View)</a>", module.Id, dockerModuleLink)
}

func createCollapsibleSection(title, content string) string {
	return fmt.Sprintf("<details><summary>%s</summary>\n%s</details>", title, content)
}
